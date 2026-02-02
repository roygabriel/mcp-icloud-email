package smtp

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/rgabriel/mcp-icloud-email/imap"
)

const (
	smtpServer = "smtp.mail.me.com"
	smtpPort   = 587
)

// Client handles SMTP operations for sending emails
type Client struct {
	username string
	password string
}

// SendOptions contains optional parameters for sending emails
type SendOptions struct {
	CC      []string
	BCC     []string
	HTML    bool
	Headers map[string]string
}

// NewClient creates a new SMTP client
func NewClient(username, password string) *Client {
	return &Client{
		username: username,
		password: password,
	}
}

// SendEmail sends an email via SMTP
func (c *Client) SendEmail(ctx context.Context, from string, to []string, subject, body string, opts SendOptions) error {
	// Create message buffer
	var buf bytes.Buffer

	// Create message header
	var h mail.Header
	h.SetDate(time.Now())
	h.SetAddressList("From", []*mail.Address{{Address: from}})

	// Set To addresses
	toAddrs := make([]*mail.Address, 0, len(to))
	for _, addr := range to {
		toAddrs = append(toAddrs, &mail.Address{Address: addr})
	}
	h.SetAddressList("To", toAddrs)

	// Set CC addresses
	if len(opts.CC) > 0 {
		ccAddrs := make([]*mail.Address, 0, len(opts.CC))
		for _, addr := range opts.CC {
			ccAddrs = append(ccAddrs, &mail.Address{Address: addr})
		}
		h.SetAddressList("Cc", ccAddrs)
	}

	// Set BCC addresses (they go in envelope but not headers)
	// BCC is intentionally NOT added to headers

	// Set subject
	h.SetSubject(subject)

	// Generate Message-ID
	messageID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), c.username, smtpServer)
	h.Set("Message-ID", messageID)

	// Set custom headers
	for key, value := range opts.Headers {
		h.Set(key, value)
	}

	// Create message writer
	var mw *mail.Writer
	var err error

	if opts.HTML {
		// Multipart alternative for HTML and plain text
		h.SetContentType("multipart/alternative", nil)
		mw, err = mail.CreateWriter(&buf, h)
		if err != nil {
			return fmt.Errorf("failed to create message writer: %w", err)
		}

		// Plain text part
		var textHeader mail.InlineHeader
		textHeader.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
		textPart, err := mw.CreateSingleInline(textHeader)
		if err != nil {
			mw.Close()
			return fmt.Errorf("failed to create text part: %w", err)
		}
		plainBody := stripHTML(body)
		if _, err := textPart.Write([]byte(plainBody)); err != nil {
			mw.Close()
			return fmt.Errorf("failed to write text part: %w", err)
		}
		textPart.Close()

		// HTML part
		var htmlHeader mail.InlineHeader
		htmlHeader.SetContentType("text/html", map[string]string{"charset": "utf-8"})
		htmlPart, err := mw.CreateSingleInline(htmlHeader)
		if err != nil {
			mw.Close()
			return fmt.Errorf("failed to create HTML part: %w", err)
		}
		if _, err := htmlPart.Write([]byte(body)); err != nil {
			mw.Close()
			return fmt.Errorf("failed to write HTML part: %w", err)
		}
		htmlPart.Close()

		mw.Close()
	} else {
		// Plain text only
		h.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
		mw, err = mail.CreateWriter(&buf, h)
		if err != nil {
			return fmt.Errorf("failed to create message writer: %w", err)
		}
		
		// Create inline part for plain text
		var textHeader mail.InlineHeader
		textHeader.SetContentType("text/plain", map[string]string{"charset": "utf-8"})
		textPart, err := mw.CreateSingleInline(textHeader)
		if err != nil {
			mw.Close()
			return fmt.Errorf("failed to create text part: %w", err)
		}
		if _, err := textPart.Write([]byte(body)); err != nil {
			mw.Close()
			return fmt.Errorf("failed to write body: %w", err)
		}
		textPart.Close()
		mw.Close()
	}

	// Build recipient list (To + CC + BCC)
	recipients := make([]string, 0, len(to)+len(opts.CC)+len(opts.BCC))
	recipients = append(recipients, to...)
	recipients = append(recipients, opts.CC...)
	recipients = append(recipients, opts.BCC...)

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", smtpServer, smtpPort)
	auth := smtp.PlainAuth("", c.username, c.password, smtpServer)

	err = smtp.SendMail(addr, auth, from, recipients, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// ReplyToEmail replies to an existing email
func (c *Client) ReplyToEmail(ctx context.Context, original *imap.Email, body string, replyAll bool, opts SendOptions) error {
	// Build recipient list
	to := []string{original.From}
	
	var cc []string
	if replyAll {
		// Add all To recipients except ourselves
		for _, addr := range original.To {
			if !strings.Contains(addr, c.username) {
				cc = append(cc, addr)
			}
		}
		// Add all CC recipients except ourselves
		for _, addr := range original.CC {
			if !strings.Contains(addr, c.username) {
				cc = append(cc, addr)
			}
		}
	}

	// Merge with provided CC
	if len(opts.CC) > 0 {
		cc = append(cc, opts.CC...)
	}

	// Build subject with Re: prefix
	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	// Build reply headers
	headers := make(map[string]string)
	if original.MessageID != "" {
		headers["In-Reply-To"] = original.MessageID
		
		// Build References header
		refs := []string{}
		if len(original.References) > 0 {
			refs = append(refs, original.References...)
		}
		refs = append(refs, original.MessageID)
		headers["References"] = strings.Join(refs, " ")
	}

	// Merge with provided headers
	for key, value := range opts.Headers {
		headers[key] = value
	}

	// Send the reply
	sendOpts := SendOptions{
		CC:      cc,
		BCC:     opts.BCC,
		HTML:    opts.HTML,
		Headers: headers,
	}

	return c.SendEmail(ctx, c.username, to, subject, body, sendOpts)
}

// stripHTML removes HTML tags for plain text version (basic implementation)
func stripHTML(html string) string {
	// Simple HTML stripping - replace common tags with newlines
	text := strings.ReplaceAll(html, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = strings.ReplaceAll(text, "</div>", "\n")
	
	// Remove remaining tags
	inTag := false
	var result strings.Builder
	for _, char := range text {
		if char == '<' {
			inTag = true
			continue
		}
		if char == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(char)
		}
	}
	
	return strings.TrimSpace(result.String())
}
