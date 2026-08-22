package email

import (
	"github.com/jackc/pgx/v5"

	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"finance-accounting-app/backend/internal/db"
)

// tlsConfigFor builds the STARTTLS config. ServerName from the host; modern
// cipher suites; verification on (production mail hosts present valid certs).
func tlsConfigFor(host string) tls.Config {
	return tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

// ---------------------------------------------------------------------------
// SMTP delivery worker.
//
// The /email/queue endpoints manage the queue; this worker performs the actual
// SMTP delivery. Configuration comes entirely from the environment:
//
//	SMTP_HOST   (required to enable; empty disables the worker)
//	SMTP_PORT   (default 587)
//	SMTP_USER   (default "")
//	SMTP_PASS   (default "")
//	SMTP_FROM   (default SMTP_USER or "no-reply@localhost")
//	SMTP_HELO   (default "localhost")
//	SMTP_INTERVAL (seconds between passes; default 60)
//
// With SMTP_HOST empty the worker logs one line and does nothing — the queue
// keeps recording send attempts exactly as before (dev/demo mode), so the
// feature stays backward compatible.
//
// Delivery rules per pass:
//   - PENDING rows with retry_count < max_retries are attempted
//   - success   → status SENT, sent_at, last_error cleared
//   - failure   → retry_count++, last_error recorded;
//                 when retries are exhausted → status FAILED
//   - a row locked FOR UPDATE SKIP LOCKED by another worker pass is skipped
// ---------------------------------------------------------------------------

// SMTPConfig holds the worker's environment-derived settings. DialTimeout
// and SessionDeadline default to 10s / 60s when zero (F-12: one slow MX must
// never freeze the sequential worker loop); tests may shorten them.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	Helo     string
	Interval time.Duration

	DialTimeout     time.Duration // TCP connect timeout; 0 → 10s
	SessionDeadline time.Duration // whole SMTP session deadline; 0 → 60s
}

// SMTPConfigFromEnv builds the config from the environment. When Host is
// empty the worker is disabled.
func SMTPConfigFromEnv() SMTPConfig {
	port := 587
	if raw := strings.TrimSpace(os.Getenv("SMTP_PORT")); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil && p > 0 && p < 65536 {
			port = p
		}
	}
	user := os.Getenv("SMTP_USER")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		if user != "" {
			from = user
		} else {
			from = "no-reply@localhost"
		}
	}
	interval := 60 * time.Second
	if raw := strings.TrimSpace(os.Getenv("SMTP_INTERVAL")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs >= 5 {
			interval = time.Duration(secs) * time.Second
		}
	}
	helo := strings.TrimSpace(os.Getenv("SMTP_HELO"))
	if helo == "" {
		helo = "localhost"
	}
	return SMTPConfig{
		Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port:     port,
		User:     user,
		Pass:     os.Getenv("SMTP_PASS"),
		From:     from,
		Helo:     helo,
		Interval: interval,
	}
}

// queuedEmail is one row claimed for delivery.
type queuedEmail struct {
	id         int64
	tenantID   int64
	to         string
	cc         string
	bcc        string
	subject    string
	bodyHTML   string
	bodyText   string
	retryCount int
	maxRetries int
}

// StartSMTPWorker launches the background delivery loop. Disabled (logged,
// no-op) when cfg.Host is empty. Stops when ctx is cancelled.
func (s *Service) StartSMTPWorker(ctx context.Context, cfg SMTPConfig) {
	if cfg.Host == "" {
		slog.Info("email smtp worker disabled (SMTP_HOST not set) — queue records attempts only")
		return
	}
	go func() {
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("email smtp worker stopped")
				return
			case <-ticker.C:
				s.RunDeliveryPass(ctx, cfg)
			}
		}
	}()
	slog.Info("email smtp worker started",
		"host", cfg.Host, "port", cfg.Port, "from", cfg.From, "interval", cfg.Interval.String())
}

// RunDeliveryPass claims and sends every deliverable row once. Exported so
// tests can drive a pass without the ticker.
func (s *Service) RunDeliveryPass(ctx context.Context, cfg SMTPConfig) (sent, failed, retried int) {
	emails, err := s.claimPending(ctx)
	if err != nil {
		slog.Error("email worker: claim failed", "error", err)
		return 0, 0, 0
	}
	for _, e := range emails {
		sendErr := SendMail(cfg, e.to, e.cc, e.bcc, e.subject, e.bodyHTML, e.bodyText)
		switch {
		case sendErr == nil:
			sent++
			s.markResult(ctx, e.tenantID, e.id, true, "")
		case e.retryCount+1 >= e.maxRetries:
			failed++
			s.markResult(ctx, e.tenantID, e.id, false, sendErr.Error())
		default:
			retried++
			s.markRetry(ctx, e.tenantID, e.id, sendErr.Error())
		}
	}
	if sent+failed+retried > 0 {
		slog.Info("email worker pass", "sent", sent, "failed", failed, "retried", retried)
	}
	return sent, failed, retried
}

// claimPending claims PENDING rows that still have retries left. The status
// flip to SENDING is atomic via UPDATE … WHERE id IN (…FOR UPDATE SKIP
// LOCKED) so concurrent workers cannot double-send. email_queue is
// RLS-scoped with fail-closed policies, so claims run per tenant (tenants
// themselves are not RLS-scoped); the per-tenant LIMIT keeps each pass fair.
func (s *Service) claimPending(ctx context.Context) ([]queuedEmail, error) {
	tenantIDs, err := s.tenantIDs(ctx)
	if err != nil {
		return nil, err
	}
	var out []queuedEmail
	for _, tenantID := range tenantIDs {
		if err := db.WithTenantData(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				UPDATE email_queue q
				SET status = 'SENDING', updated_at = now()
				WHERE q.id IN (
					SELECT id FROM email_queue
					WHERE status = 'PENDING' AND retry_count < max_retries
					ORDER BY created_at
					LIMIT 25
					FOR UPDATE SKIP LOCKED
				)
				RETURNING q.id, q.tenant_id, q.to_email, COALESCE(q.cc_email, ''), COALESCE(q.bcc_email, ''),
				          q.subject, COALESCE(q.body_html, ''), COALESCE(q.body_text, ''),
				          q.retry_count, q.max_retries
			`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var e queuedEmail
				if err := rows.Scan(&e.id, &e.tenantID, &e.to, &e.cc, &e.bcc,
					&e.subject, &e.bodyHTML, &e.bodyText, &e.retryCount, &e.maxRetries); err != nil {
					return err
				}
				out = append(out, e)
			}
			return rows.Err()
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// tenantIDs enumerates deployment tenants for cross-tenant worker passes.
func (s *Service) tenantIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// markResult records the terminal outcome of a delivery attempt. On success
// last_error is cleared; on failure the status becomes FAILED.
func (s *Service) markResult(ctx context.Context, tenantID, id int64, ok bool, errMsg string) {
	_ = db.WithTenantData(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		if ok {
			_, err := tx.Exec(ctx, `
				UPDATE email_queue
				SET status = 'SENT', sent_at = now(), last_error = NULL, updated_at = now()
				WHERE id = $1
			`, id)
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE email_queue
			SET status = 'FAILED', last_error = $2, updated_at = now()
			WHERE id = $1
		`, id, truncate(errMsg, 500))
		return err
	})
}

// markRetry schedules another attempt: back to PENDING with retry_count+1.
func (s *Service) markRetry(ctx context.Context, tenantID, id int64, errMsg string) {
	_ = db.WithTenantData(ctx, s.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE email_queue
			SET status = 'PENDING', retry_count = retry_count + 1, last_error = $2, updated_at = now()
			WHERE id = $1
		`, id, truncate(errMsg, 500))
		return err
	})
}

// SendMail delivers one message via SMTP. HTML alternative + plain text are
// sent as a multipart/alternative body so both kinds of clients render.
// It is a package-level function so tests can exercise message building.
func SendMail(cfg SMTPConfig, to, cc, bcc, subject, bodyHTML, bodyText string) error {
	if cfg.Host == "" {
		return fmt.Errorf("smtp disabled: SMTP_HOST not configured")
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	recipients := splitAddresses(to, cc, bcc)
	if len(recipients) == 0 {
		return fmt.Errorf("no recipients")
	}

	msg := buildMIME(cfg.From, recipients, subject, bodyHTML, bodyText)

	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}

	// F-12: bound the dial and the whole SMTP session. Previously smtp.Dial
	// had no timeout, so an unresponsive MX could block the sequential
	// worker loop indefinitely.
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 10 * time.Second
	}
	sessionDeadline := cfg.SessionDeadline
	if sessionDeadline <= 0 {
		sessionDeadline = 60 * time.Second
	}

	rawConn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer rawConn.Close()
	// The deadline covers the entire session (helo/tls/auth/mail/rcpt/data).
	_ = rawConn.SetDeadline(time.Now().Add(sessionDeadline))
	conn, err := smtp.NewClient(rawConn, cfg.Host)
	if err != nil {
		return fmt.Errorf("client %s: %w", addr, err)
	}

	if err := conn.Hello(cfg.Helo); err != nil {
		return fmt.Errorf("helo: %w", err)
	}
	// Opportunistic STARTTLS when the server advertises it (port 587 flow).
	if ok, _ := conn.Extension("STARTTLS"); ok {
		tlsCfg := tlsConfigFor(cfg.Host)
		if err := conn.StartTLS(&tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if auth != nil {
		if ok, _ := conn.Extension("AUTH"); ok {
			if err := conn.Auth(auth); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
		}
	}
	if err := conn.Mail(cfg.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range recipients {
		if err := conn.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return conn.Quit()
}

// buildMIME assembles a RFC 5322 message with a multipart/alternative body.
func buildMIME(from string, recipients []string, subject, bodyHTML, bodyText string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + recipients[0] + "\r\n")
	b.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString(`Content-Type: multipart/alternative; boundary="finapp-mail"` + "\r\n")
	b.WriteString("\r\n")
	b.WriteString("--finapp-mail\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(bodyText + "\r\n\r\n")
	b.WriteString("--finapp-mail\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(bodyHTML + "\r\n\r\n")
	b.WriteString("--finapp-mail--\r\n")
	return []byte(b.String())
}

// splitAddresses flattens comma/semicolon separated address lists, dropping
// empties and duplicates.
func splitAddresses(lists ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, raw := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ';' }) {
			addr := strings.TrimSpace(raw)
			if addr == "" || seen[addr] {
				continue
			}
			seen[addr] = true
			out = append(out, addr)
		}
	}
	return out
}

// sanitizeHeader strips CR/LF from a header value to prevent injection.
// CRLF collapses into a single space so folded headers stay readable.
func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r\n", " ")
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
