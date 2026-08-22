package email

import (
	"net"
	"strings"
	"testing"
	"time"
)

func TestSplitAddresses(t *testing.T) {
	tests := []struct {
		name  string
		lists []string
		want  []string
	}{
		{name: "single", lists: []string{"a@x.com"}, want: []string{"a@x.com"}},
		{name: "comma separated", lists: []string{"a@x.com, b@x.com"}, want: []string{"a@x.com", "b@x.com"}},
		{name: "semicolon separated", lists: []string{"a@x.com; b@x.com"}, want: []string{"a@x.com", "b@x.com"}},
		{
			name:  "across to/cc/bcc",
			lists: []string{"a@x.com", "b@x.com, c@x.com", "a@x.com, d@x.com"},
			want:  []string{"a@x.com", "b@x.com", "c@x.com", "d@x.com"},
		},
		{name: "drops empties and whitespace", lists: []string{" , a@x.com , ,"}, want: []string{"a@x.com"}},
		{name: "all empty", lists: []string{"", " "}, want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitAddresses(tc.lists...)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("addr[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSanitizeHeader(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "Quarterly Invoice", want: "Quarterly Invoice"},
		{in: "Injected\r\nBcc: evil@x.com", want: "Injected Bcc: evil@x.com"},
		{in: "Line\nBreak", want: "Line Break"},
		{in: "  padded  ", want: "padded"},
	}
	for _, tc := range tests {
		if got := sanitizeHeader(tc.in); got != tc.want {
			t.Errorf("sanitizeHeader(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildMIME(t *testing.T) {
	msg := string(buildMIME("no-reply@fin.app", []string{"cust@x.com"}, "Subject Line", "<p>Hi</p>", "Hi"))
	if !strings.Contains(msg, "From: no-reply@fin.app\r\n") {
		t.Error("missing From header")
	}
	if !strings.Contains(msg, "To: cust@x.com\r\n") {
		t.Error("missing To header")
	}
	if !strings.Contains(msg, "Subject: Subject Line\r\n") {
		t.Error("missing Subject header")
	}
	if !strings.Contains(msg, "multipart/alternative") {
		t.Error("missing multipart Content-Type")
	}
	if !strings.Contains(msg, "text/plain") || !strings.Contains(msg, "text/html") {
		t.Error("missing alternative parts")
	}
	if !strings.HasSuffix(msg, "--finapp-mail--\r\n") {
		t.Error("missing closing boundary")
	}
}

func TestSMTPConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("SMTP_INTERVAL", "")

	cfg := SMTPConfigFromEnv()
	if cfg.Host != "" {
		t.Errorf("host should default empty, got %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("port should default 587, got %d", cfg.Port)
	}
	if cfg.From != "no-reply@localhost" {
		t.Errorf("from should default no-reply@localhost, got %q", cfg.From)
	}
	if cfg.Interval != 60_000_000_000 { // 60s in ns
		t.Errorf("interval should default 60s, got %v", cfg.Interval)
	}
}

func TestSMTPConfigFromEnvFull(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USER", "mailer@example.com")
	t.Setenv("SMTP_PASS", "secret")
	t.Setenv("SMTP_FROM", "billing@example.com")
	t.Setenv("SMTP_HELO", "finapp.local")
	t.Setenv("SMTP_INTERVAL", "120")

	cfg := SMTPConfigFromEnv()
	if cfg.Host != "smtp.example.com" || cfg.Port != 465 {
		t.Errorf("host/port = %q/%d", cfg.Host, cfg.Port)
	}
	if cfg.From != "billing@example.com" {
		t.Errorf("from = %q", cfg.From)
	}
	if cfg.Helo != "finapp.local" {
		t.Errorf("helo = %q", cfg.Helo)
	}
	if cfg.Interval != 120_000_000_000 {
		t.Errorf("interval = %v", cfg.Interval)
	}
}

func TestSMTPConfigFromEnvInvalidPort(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "not-a-port")
	cfg := SMTPConfigFromEnv()
	if cfg.Port != 587 {
		t.Errorf("invalid port must fall back to 587, got %d", cfg.Port)
	}
}

func TestSendMailDisabled(t *testing.T) {
	err := SendMail(SMTPConfig{}, "a@x.com", "", "", "s", "<p>b</p>", "b")
	if err == nil || !strings.Contains(err.Error(), "SMTP_HOST not configured") {
		t.Errorf("expected disabled error, got %v", err)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate should keep short strings, got %q", got)
	}
	long := strings.Repeat("x", 600)
	if got := truncate(long, 500); len(got) != 500 {
		t.Errorf("truncate should cap at n, got %d", len(got))
	}
}

// F-12: an SMTP server that accepts the TCP connection but never answers
// must not freeze SendMail. The session deadline (default 60s, shortened
// here) must abort it well under a worker-hostile hang; in production the
// timeout error feeds the existing retry queue path.
func TestSendMail_SilentServerTimesOut(t *testing.T) {
	// Listener that accepts connections but never writes a byte.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	var held []net.Conn
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				for _, c := range held {
					_ = c.Close()
				}
				return
			}
			// Hold the connection open, silent.
			held = append(held, conn)
		}
	}()

	cfg := SMTPConfig{
		Host:            "127.0.0.1",
		Port:            listener.Addr().(*net.TCPAddr).Port,
		From:            "no-reply@test.local",
		Helo:            "localhost",
		DialTimeout:     500 * time.Millisecond,
		SessionDeadline: 800 * time.Millisecond,
	}

	start := time.Now()
	err = SendMail(cfg, "dest@x.com", "", "", "subject", "<p>b</p>", "b")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error against a silent server")
	}
	if elapsed > 5*time.Second {
		t.Errorf("SendMail took %s, want well under the 60s default deadline", elapsed)
	}
}
