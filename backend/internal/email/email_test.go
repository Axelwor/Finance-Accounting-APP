package email

import (
	"testing"
)

// TestValidateTemplate covers the pure validation for create-template
// requests: code, name, subject, body_html, and trigger_event are all
// required (non-empty after trimming).
func TestValidateTemplate(t *testing.T) {
	t.Parallel()
	type tc struct {
		name         string
		code         string
		displayName  string
		subject      string
		bodyHTML     string
		triggerEvent string
		wantCode     string
		wantMsg      string
	}
	cases := []tc{
		{
			name:         "valid template",
			code:         "TPL-001",
			displayName:  "Invoice Reminder",
			subject:      "Your invoice is due",
			bodyHTML:     "<p>Reminder</p>",
			triggerEvent: "INVOICE_DUE",
			wantCode:     "",
			wantMsg:      "",
		},
		{
			name:         "valid with whitespace trimmed",
			code:         "  TPL-002  ",
			displayName:  "  Welcome  ",
			subject:      "  Welcome aboard  ",
			bodyHTML:     "  <b>Hi</b>  ",
			triggerEvent: "  USER_REGISTERED  ",
			wantCode:     "",
			wantMsg:      "",
		},
		{
			name:         "missing code",
			code:         "",
			displayName:  "Welcome",
			subject:      "Hi",
			bodyHTML:     "<p>x</p>",
			triggerEvent: "USER_REGISTERED",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "code is required",
		},
		{
			name:         "whitespace code",
			code:         "   ",
			displayName:  "Welcome",
			subject:      "Hi",
			bodyHTML:     "<p>x</p>",
			triggerEvent: "USER_REGISTERED",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "code is required",
		},
		{
			name:         "missing name",
			code:         "TPL-001",
			displayName:  "",
			subject:      "Hi",
			bodyHTML:     "<p>x</p>",
			triggerEvent: "USER_REGISTERED",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "name is required",
		},
		{
			name:         "missing subject",
			code:         "TPL-001",
			displayName:  "Welcome",
			subject:      "",
			bodyHTML:     "<p>x</p>",
			triggerEvent: "USER_REGISTERED",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "subject is required",
		},
		{
			name:         "missing body_html",
			code:         "TPL-001",
			displayName:  "Welcome",
			subject:      "Hi",
			bodyHTML:     "",
			triggerEvent: "USER_REGISTERED",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "body_html is required",
		},
		{
			name:         "missing trigger_event",
			code:         "TPL-001",
			displayName:  "Welcome",
			subject:      "Hi",
			bodyHTML:     "<p>x</p>",
			triggerEvent: "",
			wantCode:     "INVALID_REQUEST",
			wantMsg:      "trigger_event is required",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotCode, gotMsg := validateTemplate(c.code, c.displayName, c.subject, c.bodyHTML, c.triggerEvent)
			if gotCode != c.wantCode {
				t.Fatalf("code = %q, want %q", gotCode, c.wantCode)
			}
			if gotMsg != c.wantMsg {
				t.Fatalf("msg = %q, want %q", gotMsg, c.wantMsg)
			}
		})
	}
}

// TestValidateEnqueue covers the pure validation rules for enqueue requests:
// to_email is always required, and when no valid template_id is provided, a
// subject must be supplied instead.
func TestValidateEnqueue(t *testing.T) {
	t.Parallel()
	i64 := func(v int64) *int64 { return &v }
	strPtr := func(s string) *string { return &s }

	type tc struct {
		name     string
		req      EnqueueRequest
		wantCode string
		wantMsg  string
	}
	cases := []tc{
		{
			name:     "valid with template_id",
			req:      EnqueueRequest{TemplateID: i64(1), ToEmail: "user@example.com"},
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:     "valid with subject instead of template",
			req:      EnqueueRequest{ToEmail: "user@example.com", Subject: strPtr("Hello")},
			wantCode: "",
			wantMsg:  "",
		},
		{
			name:     "missing to_email with template",
			req:      EnqueueRequest{TemplateID: i64(1), ToEmail: ""},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "to_email is required",
		},
		{
			name:     "whitespace to_email",
			req:      EnqueueRequest{TemplateID: i64(1), ToEmail: "   "},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "to_email is required",
		},
		{
			name:     "no template and no subject",
			req:      EnqueueRequest{ToEmail: "user@example.com"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "either template_id or subject is required",
		},
		{
			name:     "no template and whitespace subject",
			req:      EnqueueRequest{ToEmail: "user@example.com", Subject: strPtr("   ")},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "either template_id or subject is required",
		},
		{
			name:     "zero template_id and no subject",
			req:      EnqueueRequest{TemplateID: i64(0), ToEmail: "user@example.com"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "either template_id or subject is required",
		},
		{
			name:     "negative template_id and no subject",
			req:      EnqueueRequest{TemplateID: i64(-3), ToEmail: "user@example.com"},
			wantCode: "INVALID_REQUEST",
			wantMsg:  "either template_id or subject is required",
		},
		{
			name:     "zero template_id with subject is valid",
			req:      EnqueueRequest{TemplateID: i64(0), ToEmail: "user@example.com", Subject: strPtr("Direct")},
			wantCode: "",
			wantMsg:  "",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			gotCode, gotMsg := validateEnqueue(c.req)
			if gotCode != c.wantCode {
				t.Fatalf("code = %q, want %q", gotCode, c.wantCode)
			}
			if gotMsg != c.wantMsg {
				t.Fatalf("msg = %q, want %q", gotMsg, c.wantMsg)
			}
		})
	}
}

// TestCanTransitionEmailStatus verifies the queue state-transition guard:
// only PENDING items may move to SENT or CANCELLED; all other transitions
// (including from terminal states) are rejected.
func TestCanTransitionEmailStatus(t *testing.T) {
	t.Parallel()
	type tc struct {
		name    string
		current string
		target  string
		want    bool
	}
	cases := []tc{
		{name: "pending to sent", current: "PENDING", target: "SENT", want: true},
		{name: "pending to cancelled", current: "PENDING", target: "CANCELLED", want: true},
		{name: "sent to cancelled", current: "SENT", target: "CANCELLED", want: false},
		{name: "sent to sent", current: "SENT", target: "SENT", want: false},
		{name: "cancelled to sent", current: "CANCELLED", target: "SENT", want: false},
		{name: "cancelled to pending", current: "CANCELLED", target: "PENDING", want: false},
		{name: "pending to pending", current: "PENDING", target: "PENDING", want: false},
		{name: "pending to unknown", current: "PENDING", target: "FROZEN", want: false},
		{name: "empty current", current: "", target: "SENT", want: false},
		{name: "empty target", current: "PENDING", target: "", want: false},
		{name: "lowercase pending to sent", current: "pending", target: "sent", want: true},
		{name: "padded pending to cancelled", current: "  pending ", target: " cancelled ", want: true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := canTransitionEmailStatus(c.current, c.target); got != c.want {
				t.Fatalf("canTransitionEmailStatus(%q, %q) = %v, want %v", c.current, c.target, got, c.want)
			}
		})
	}
}

// TestEmailStatusConstants verifies the queue status string constants.
func TestEmailStatusConstants(t *testing.T) {
	t.Parallel()
	type tc struct {
		name string
		got  string
		want string
	}
	cases := []tc{
		{name: "EmailStatusPending", got: EmailStatusPending, want: "PENDING"},
		{name: "EmailStatusSent", got: EmailStatusSent, want: "SENT"},
		{name: "EmailStatusCancelled", got: EmailStatusCancelled, want: "CANCELLED"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if c.got != c.want {
				t.Fatalf("constant %s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}

// TestPathID covers the URL path-ID parser used by every handler.
func TestPathID(t *testing.T) {
	t.Parallel()
	type tc struct {
		name string
		raw  string
		want int64
	}
	cases := []tc{
		{name: "positive integer", raw: "42", want: 42},
		{name: "one", raw: "1", want: 1},
		{name: "zero", raw: "0", want: 0},
		{name: "negative", raw: "-5", want: -5},
		{name: "non-numeric", raw: "abc", want: 0},
		{name: "empty", raw: "", want: 0},
		{name: "float string", raw: "1.5", want: 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := pathID(c.raw); got != c.want {
				t.Fatalf("pathID(%q) = %d, want %d", c.raw, got, c.want)
			}
		})
	}
}
