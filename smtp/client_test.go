package smtp

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"

	"github.com/rgabriel/mcp-icloud-email/imap"
)

// --- Helpers ---

type capturedSend struct {
	addr       string
	from       string
	recipients []string
	msg        []byte
}

func newTestClient(capture *capturedSend, sendErr error) *Client {
	return &Client{
		username: "test@icloud.com",
		password: "test-password",
		sendMail: func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			if capture != nil {
				capture.addr = addr
				capture.from = from
				capture.recipients = to
				capture.msg = msg
			}
			return sendErr
		},
	}
}

// --- stripHTML tests ---

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "br variants",
			input: "line1<br>line2<br/>line3<br />line4",
			want:  "line1\nline2\nline3\nline4",
		},
		{
			name:  "p and div tags",
			input: "<p>paragraph1</p><div>block1</div>",
			want:  "paragraph1\n\nblock1",
		},
		{
			name:  "nested tags",
			input: "<div><p><b>bold</b> text</p></div>",
			want:  "bold text",
		},
		{
			name:  "plain text passthrough",
			input: "just plain text",
			want:  "just plain text",
		},
		{
			name:  "complex HTML",
			input: "<html><body><h1>Title</h1><p>Hello <a href=\"#\">World</a></p></body></html>",
			want:  "TitleHello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- SendEmail tests ---

func TestSendEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("plain text", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		err := c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "Hello", "Body text", SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if cap.from != "test@icloud.com" {
			t.Errorf("from = %q", cap.from)
		}
		if len(cap.recipients) != 1 || cap.recipients[0] != "to@example.com" {
			t.Errorf("recipients = %v", cap.recipients)
		}
		msgStr := string(cap.msg)
		if !strings.Contains(msgStr, "Hello") {
			t.Error("message should contain subject")
		}
		if !strings.Contains(msgStr, "Body text") {
			t.Error("message should contain body")
		}
		// Message-ID header uses MIME key casing
		if !strings.Contains(msgStr, "Message-Id") && !strings.Contains(msgStr, "Message-ID") {
			t.Error("message should contain Message-ID header")
		}
	})

	t.Run("HTML multipart", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		err := c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "HTML Test", "<p>Hello</p>", SendOptions{HTML: true})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		msgStr := string(cap.msg)
		// go-message library produces multipart content
		msgLower := strings.ToLower(msgStr)
		if !strings.Contains(msgLower, "multipart/") {
			t.Errorf("HTML email should be multipart, got:\n%s", msgStr[:min(500, len(msgStr))])
		}
		if !strings.Contains(msgStr, "<p>Hello</p>") {
			t.Error("message should contain HTML body")
		}
		// The stripped text part should also be present
		if !strings.Contains(msgStr, "Hello") {
			t.Error("message should contain plain text version")
		}
	})

	t.Run("CC and BCC recipients", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		err := c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", SendOptions{
			CC:  []string{"cc@example.com"},
			BCC: []string{"bcc@example.com"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Recipients should include to + cc + bcc
		if len(cap.recipients) != 3 {
			t.Errorf("recipients count = %d, want 3, got %v", len(cap.recipients), cap.recipients)
		}
		msgStr := string(cap.msg)
		// CC should be in headers
		if !strings.Contains(msgStr, "cc@example.com") {
			t.Error("message should contain CC address")
		}
		// BCC should NOT be in headers
		if strings.Contains(msgStr, "bcc@example.com") {
			t.Error("BCC should not appear in message headers")
		}
	})

	t.Run("custom headers", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		err := c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", SendOptions{
			Headers: map[string]string{
				"In-Reply-To": "<original@example.com>",
				"References":  "<original@example.com>",
			},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		msgStr := string(cap.msg)
		if !strings.Contains(msgStr, "In-Reply-To") {
			t.Error("message should contain In-Reply-To header")
		}
	})

	t.Run("send error propagation", func(t *testing.T) {
		c := newTestClient(nil, errors.New("SMTP connection refused"))

		err := c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", SendOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to send email") {
			t.Errorf("error = %q, want containing 'failed to send email'", err.Error())
		}
	})

	t.Run("Message-ID generation", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		_ = c.SendEmail(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", SendOptions{})
		msgStr := string(cap.msg)
		if !strings.Contains(msgStr, "@smtp.mail.me.com>") {
			t.Error("Message-ID should contain SMTP server domain")
		}
		if !strings.Contains(msgStr, "test@icloud.com") {
			t.Error("Message-ID should contain username")
		}
	})
}

// --- ReplyToEmail tests ---

func TestReplyToEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("basic reply", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:      "sender@example.com",
			To:        []string{"test@icloud.com"},
			Subject:   "Original Subject",
			MessageID: "<original@example.com>",
		}

		err := c.ReplyToEmail(ctx, original, "Reply body", false, SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Should send to original sender
		if len(cap.recipients) != 1 || cap.recipients[0] != "sender@example.com" {
			t.Errorf("recipients = %v", cap.recipients)
		}
		msgStr := string(cap.msg)
		if !strings.Contains(msgStr, "Re: Original Subject") {
			t.Error("subject should have Re: prefix")
		}
		if !strings.Contains(msgStr, "In-Reply-To") {
			t.Error("should have In-Reply-To header")
		}
		if !strings.Contains(msgStr, "References") {
			t.Error("should have References header")
		}
	})

	t.Run("Re: prefix already present", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:      "sender@example.com",
			Subject:   "Re: Already replied",
			MessageID: "<msg@example.com>",
		}

		err := c.ReplyToEmail(ctx, original, "Another reply", false, SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		msgStr := string(cap.msg)
		// Should not duplicate Re:
		if strings.Contains(msgStr, "Re: Re:") {
			t.Error("should not duplicate Re: prefix")
		}
	})

	t.Run("reply all filtering self", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:      "sender@example.com",
			To:        []string{"test@icloud.com", "other@example.com"},
			CC:        []string{"cc@example.com", "test@icloud.com"},
			Subject:   "Group Thread",
			MessageID: "<group@example.com>",
		}

		err := c.ReplyToEmail(ctx, original, "Reply to all", true, SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Recipients: to=[sender@example.com], cc=[other@example.com, cc@example.com]
		// Self (test@icloud.com) should be excluded from CC
		for _, r := range cap.recipients {
			if strings.Contains(r, "test@icloud.com") {
				t.Errorf("self should be filtered from recipients, got %v", cap.recipients)
			}
		}
		// other@example.com and cc@example.com should be present
		recipStr := strings.Join(cap.recipients, ",")
		if !strings.Contains(recipStr, "other@example.com") {
			t.Error("other@example.com should be in recipients")
		}
		if !strings.Contains(recipStr, "cc@example.com") {
			t.Error("cc@example.com should be in recipients")
		}
	})

	t.Run("CC merging", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:      "sender@example.com",
			To:        []string{"test@icloud.com"},
			Subject:   "Test",
			MessageID: "<msg@example.com>",
		}

		err := c.ReplyToEmail(ctx, original, "Reply", false, SendOptions{
			CC: []string{"extra-cc@example.com"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Should include extra CC
		recipStr := strings.Join(cap.recipients, ",")
		if !strings.Contains(recipStr, "extra-cc@example.com") {
			t.Error("extra CC should be included")
		}
	})

	t.Run("with existing references", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:       "sender@example.com",
			Subject:    "Thread",
			MessageID:  "<msg-3@example.com>",
			References: []string{"<msg-1@example.com>", "<msg-2@example.com>"},
		}

		err := c.ReplyToEmail(ctx, original, "Reply", false, SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		msgStr := string(cap.msg)
		// References should include all originals plus the message being replied to
		if !strings.Contains(msgStr, "<msg-1@example.com>") {
			t.Error("References should include original refs")
		}
		if !strings.Contains(msgStr, "<msg-3@example.com>") {
			t.Error("References should include replied-to message ID")
		}
	})

	t.Run("no message ID", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:    "sender@example.com",
			Subject: "No MsgID",
		}

		err := c.ReplyToEmail(ctx, original, "Reply", false, SendOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Should still work, just without In-Reply-To/References
	})

	t.Run("send error propagation", func(t *testing.T) {
		c := newTestClient(nil, errors.New("SMTP error"))

		original := &imap.Email{
			From:    "sender@example.com",
			Subject: "Test",
		}

		err := c.ReplyToEmail(ctx, original, "Reply", false, SendOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("reply with custom headers", func(t *testing.T) {
		var cap capturedSend
		c := newTestClient(&cap, nil)

		original := &imap.Email{
			From:      "sender@example.com",
			Subject:   "Test",
			MessageID: "<msg@example.com>",
		}

		err := c.ReplyToEmail(ctx, original, "Reply", false, SendOptions{
			Headers: map[string]string{"X-Custom": "value"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		msgStr := string(cap.msg)
		if !strings.Contains(msgStr, "X-Custom") {
			t.Error("custom headers should be merged")
		}
	})
}
