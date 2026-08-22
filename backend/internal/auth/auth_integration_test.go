package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// F-06: atomic refresh-token rotation + replay detection.
//
// These paths need a live Postgres (user_tokens rows, row locks) and are
// skipped unless TEST_DATABASE_URL is set — same convention as internal/db
// and internal/reporting integration tests. Seed data uses per-run unique
// names and best-effort cleanup.
// ---------------------------------------------------------------------------

func newAuthTestEnv(t *testing.T, ctx context.Context) (*Service, *pgxpool.Pool, int64, func()) {
	t.Helper()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := "auth-rot-" + suffix + "@test.local"

	var userID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name) VALUES ($1, 'x', 'Rotation Test') RETURNING id`,
		email).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	cleanup := func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM user_tokens WHERE user_id = $1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM user_tenants WHERE user_id = $1`, userID)
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = $1`, userID)
		pool.Close()
	}
	return NewService(pool, "test-secret-that-is-long-enough-32ch"), pool, userID, cleanup
}

// insertRefreshToken seeds one active refresh-token row and returns its id
// plus the raw bearer value.
func insertRefreshToken(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID int64, family string) (int64, string) {
	t.Helper()
	raw := "raw-refresh-" + family
	var id int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO user_tokens (user_id, token_type, token_hash, family_id, expires_at, tenant_id, role)
		VALUES ($1, 'refresh', $2, $3, now() + interval '30 days', 0, 'owner')
		RETURNING id
	`, userID, hashToken(raw), family).Scan(&id); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	return id, raw
}

// familyState returns (active, revoked) row counts for one family.
func familyState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, family string) (int, int) {
	t.Helper()
	var active, revoked int
	if err := pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE revoked_at IS NULL),
		  COUNT(*) FILTER (WHERE revoked_at IS NOT NULL)
		FROM user_tokens WHERE family_id = $1
	`, family).Scan(&active, &revoked); err != nil {
		t.Fatalf("family state: %v", err)
	}
	return active, revoked
}

// (a) A successful rotation revokes the old row atomically and stores the
// replacement with replaced_by pointing back at it.
func TestRotateRefreshToken_Success(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	service, pool, userID, cleanup := newAuthTestEnv(t, ctx)
	defer cleanup()

	family := randomUUID()
	oldID, _ := insertRefreshToken(t, ctx, pool, userID, family)

	newRaw, newFamily, err := service.rotateRefreshToken(ctx, oldID, userID, 0, RoleOwner, family, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newFamily != family {
		t.Errorf("newFamily = %q, want same family %q", newFamily, family)
	}

	active, revoked := familyState(t, ctx, pool, family)
	if active != 1 || revoked != 1 {
		t.Fatalf("family = (%d active, %d revoked), want (1, 1)", active, revoked)
	}
	if newRaw == "" {
		t.Error("expected a fresh raw token")
	}

	var revokedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at FROM user_tokens WHERE id = $1`, oldID,
	).Scan(&revokedAt); err != nil {
		t.Fatal(err)
	}
	if revokedAt == nil {
		t.Error("old token not revoked after rotation")
	}

	// replaced_by lives on the SUCCESSOR row and points back at the old one.
	var successorID int64
	var successorReplacedBy *int64
	if err := pool.QueryRow(ctx, `
		SELECT id, replaced_by FROM user_tokens
		WHERE family_id = $1 AND id != $2 AND token_type = 'refresh'
	`, family, oldID).Scan(&successorID, &successorReplacedBy); err != nil {
		t.Fatalf("find successor: %v", err)
	}
	if successorReplacedBy == nil || *successorReplacedBy != oldID {
		t.Errorf("successor replaced_by = %v, want old id %d", successorReplacedBy, oldID)
	}
}

// (b) Presenting an already-revoked token (replay) returns ErrRefreshReuse
// AND revokes every remaining member of the family.
func TestRotateRefreshToken_ReplayRevokesFamily(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	service, pool, userID, cleanup := newAuthTestEnv(t, ctx)
	defer cleanup()

	family := randomUUID()
	oldID, _ := insertRefreshToken(t, ctx, pool, userID, family)

	// First rotation succeeds; attacker replays the OLD token afterwards.
	if _, _, err := service.rotateRefreshToken(ctx, oldID, userID, 0, RoleOwner, family, "127.0.0.1", "test"); err != nil {
		t.Fatalf("first rotate: %v", err)
	}

	_, _, err := service.rotateRefreshToken(ctx, oldID, userID, 0, RoleOwner, family, "127.0.0.1", "replayed")
	if err == nil {
		t.Fatal("expected error when rotating an already-revoked token")
	}
	if err != ErrRefreshReuse {
		t.Errorf("err = %v, want ErrRefreshReuse", err)
	}

	active, revoked := familyState(t, ctx, pool, family)
	if active != 0 {
		t.Errorf("after replay: %d tokens still active in family, want 0", active)
	}
	if revoked < 2 {
		t.Errorf("after replay: only %d tokens revoked, want >= 2 (old + rotated successor)", revoked)
	}
}

// The Refresh handler answers a replayed token with the SAME generic 401 as
// any other bad token — no reason leakage (F-06).
func TestRefreshHandler_ReplayGeneric401(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	service, pool, userID, cleanup := newAuthTestEnv(t, ctx)
	defer cleanup()

	family := randomUUID()
	_, raw := insertRefreshToken(t, ctx, pool, userID, family)

	doRefresh := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(RefreshRequest{RefreshToken: token})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		service.Refresh(rr, req)
		return rr
	}

	first := doRefresh(raw)
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh = %d, want 200", first.Code)
	}

	replay := doRefresh(raw)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("replayed refresh = %d, want 401", replay.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (%s)", err, replay.Body.String())
	}
	if resp.Code != "INVALID_REFRESH" {
		t.Errorf("code = %q, want INVALID_REFRESH", resp.Code)
	}

	// Presenting a revoked token is theft evidence: the handler must have
	// killed the whole family — the successor issued by the first rotation
	// no longer works either.
	active, revoked := familyState(t, ctx, pool, family)
	if active != 0 {
		t.Errorf("after replay: %d tokens still active in family, want 0", active)
	}
	if revoked < 2 {
		t.Errorf("after replay: only %d tokens revoked, want >= 2 (old + rotated successor)", revoked)
	}
}

// (c) Concurrent rotations of the same token: exactly one wins; losers get
// ErrRefreshReuse; the family ends up fully revoked.
func TestRotateRefreshToken_ConcurrentExactlyOneWinner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	service, pool, userID, cleanup := newAuthTestEnv(t, ctx)
	defer cleanup()

	family := randomUUID()
	oldID, _ := insertRefreshToken(t, ctx, pool, userID, family)

	const racers = 8
	var win, reuse, otherErr atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := service.rotateRefreshToken(ctx, oldID, userID, 0, RoleOwner, family, "127.0.0.1", "race")
			switch {
			case err == nil:
				win.Add(1)
			case err == ErrRefreshReuse:
				reuse.Add(1)
			default:
				otherErr.Add(1)
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if win.Load() != 1 {
		t.Errorf("winners = %d, want exactly 1", win.Load())
	}
	if reuse.Load() != racers-1 {
		t.Errorf("ErrRefreshReuse count = %d, want %d", reuse.Load(), racers-1)
	}
	// The losers' reuse detection revokes the ENTIRE family — including the
	// winner's fresh successor. That is the intended F-06 tradeoff: a raced
	// (duplicated) refresh forces the client back to login because a token
	// chain that races itself is treated as compromised.
	active, revoked := familyState(t, ctx, pool, family)
	if active != 0 {
		t.Errorf("active tokens after race = %d, want 0 (family fully revoked)", active)
	}
	if revoked < 2 {
		t.Errorf("revoked tokens after race = %d, want >= 2 (old + winner successor)", revoked)
	}
}

// F-15: registration issues owner claims consistent with the membership row.
func TestRegisterIssuesOwnerRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	connectionString := os.Getenv("TEST_DATABASE_URL")
	if connectionString == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	poolCfg, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := "register-owner-" + suffix + "@test.local"
	tenantName := "Register Owner " + suffix

	service := NewService(pool, "test-secret-that-is-long-enough-32ch")
	body, _ := json.Marshal(RegisterRequest{
		Email:      email,
		Password:   "OwnerPass!2026",
		FullName:   "Owner Role Test",
		TenantName: tenantName,
	})
	rr := httptest.NewRecorder()
	service.Register(rr, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("register = %d, want 201 (%s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Role         string `json:"role"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Role != RoleOwner {
		t.Errorf("role = %q, want %q", resp.Role, RoleOwner)
	}

	claims, err := service.parseToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if claims.Role != RoleOwner {
		t.Errorf("token role = %q, want %q", claims.Role, RoleOwner)
	}

	// Membership row must agree with the claim.
	var membershipRole string
	if err := pool.QueryRow(ctx,
		`SELECT role FROM user_tenants ut JOIN users u ON u.id = ut.user_id WHERE u.email = $1`,
		email).Scan(&membershipRole); err != nil {
		t.Fatal(err)
	}
	if membershipRole != RoleOwner {
		t.Errorf("membership role = %q, want %q", membershipRole, RoleOwner)
	}

	// Best-effort cleanup of the registration artifacts.
	bg := context.Background()
	for _, stmt := range []string{
		`DELETE FROM user_tokens WHERE user_id IN (SELECT id FROM users WHERE email = $1)`,
		`DELETE FROM journal_lines WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM journal_entries WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM ledger_chain_heads WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM outbox_events WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM accounts WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM categories WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM accounting_periods WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM report_frameworks WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM dimensions WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM dashboard_widgets WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM dashboard_layouts WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM user_tenants WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $2)`,
		`DELETE FROM users WHERE email = $1`,
		`DELETE FROM tenants WHERE slug = $2`,
	} {
		_, _ = pool.Exec(bg, stmt, email, slugify(tenantName))
	}
}
