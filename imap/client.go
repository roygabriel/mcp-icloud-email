package imap

import (
	"context"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	message "github.com/emersion/go-message/mail"
)

const (
	imapServer = "imap.mail.me.com"
	imapPort   = 993
	timeout    = 30 * time.Second
)

// Client wraps the IMAP client with iCloud-specific functionality
type Client struct {
	client   *client.Client
	username string
}

// Email represents a complete email message
type Email struct {
	ID          string       `json:"id"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	CC          []string     `json:"cc"`
	BCC         []string     `json:"bcc"`
	Subject     string       `json:"subject"`
	Date        time.Time    `json:"date"`
	BodyPlain   string       `json:"bodyPlain,omitempty"`
	BodyHTML    string       `json:"bodyHTML,omitempty"`
	Snippet     string       `json:"snippet,omitempty"`
	Unread      bool         `json:"unread"`
	Attachments []Attachment `json:"attachments,omitempty"`
	MessageID   string       `json:"messageId,omitempty"`
	References  []string     `json:"references,omitempty"`
}

// Attachment represents an email attachment
type Attachment struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// EmailFilters contains filter options for searching emails
type EmailFilters struct {
	LastDays   int
	Since      *time.Time
	Before     *time.Time
	UnreadOnly bool
	Limit      int
}

// NewClient creates a new IMAP client configured for iCloud
func NewClient(email, password string) (*Client, error) {
	// Connect to iCloud IMAP server with TLS
	addr := fmt.Sprintf("%s:%d", imapServer, imapPort)
	c, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
	if err := c.Login(email, password); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	// Test connection by selecting INBOX
	if _, err := c.Select("INBOX", false); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to select INBOX: %w", err)
	}

	return &Client{
		client:   c,
		username: email,
	}, nil
}

// Close closes the IMAP connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Logout()
	}
	return nil
}

// ListFolders lists all available mailboxes/folders
func (c *Client) ListFolders(ctx context.Context) ([]string, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	
	go func() {
		done <- c.client.List("", "*", mailboxes)
	}()

	folders := []string{}
	for m := range mailboxes {
		folders = append(folders, m.Name)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}

	return folders, nil
}

// SearchEmails searches for emails in a folder with filters
func (c *Client) SearchEmails(ctx context.Context, folder, query string, filters EmailFilters) ([]Email, error) {
	// Select the mailbox
	if _, err := c.client.Select(folder, false); err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Build search criteria
	criteria := imap.NewSearchCriteria()

	// Apply date filters
	if filters.Since != nil {
		criteria.Since = *filters.Since
	} else if filters.LastDays > 0 {
		since := time.Now().AddDate(0, 0, -filters.LastDays)
		criteria.Since = since
	}

	if filters.Before != nil {
		criteria.Before = *filters.Before
	}

	// Apply unread filter
	if filters.UnreadOnly {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	// Apply text search if provided
	if query != "" {
		criteria.Text = []string{query}
	}

	// Search for messages
	uids, err := c.client.UidSearch(criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}

	if len(uids) == 0 {
		return []Email{}, nil
	}

	// Apply limit
	if filters.Limit > 0 && len(uids) > filters.Limit {
		// Get most recent emails (highest UIDs)
		uids = uids[len(uids)-filters.Limit:]
	}

	// Create sequence set
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uids...)

	// Fetch envelope and flags for the messages
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.client.UidFetch(seqSet, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}, messages)
	}()

	emails := []Email{}
	for msg := range messages {
		email := c.parseMessageData(msg, false)
		if email != nil {
			emails = append(emails, *email)
		}
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	return emails, nil
}

// GetEmail retrieves a full email by UID
func (c *Client) GetEmail(ctx context.Context, folder, emailID string) (*Email, error) {
	// Select the mailbox
	if _, err := c.client.Select(folder, false); err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Parse UID
	var uid uint32
	if _, err := fmt.Sscanf(emailID, "%d", &uid); err != nil {
		return nil, fmt.Errorf("invalid email ID format: %w", err)
	}

	// Create sequence set
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// Fetch full message
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	section := &imap.BodySectionName{}
	go func() {
		done <- c.client.UidFetch(seqSet, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, section.FetchItem()}, messages)
	}()

	msg := <-messages
	if msg == nil {
		<-done
		return nil, fmt.Errorf("email not found")
	}

	email := c.parseMessageData(msg, true)
	
	if err := <-done; err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}

	if email == nil {
		return nil, fmt.Errorf("failed to parse email")
	}

	return email, nil
}

// CountEmails counts emails matching filters
func (c *Client) CountEmails(ctx context.Context, folder string, filters EmailFilters) (int, error) {
	// Select the mailbox
	if _, err := c.client.Select(folder, false); err != nil {
		return 0, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Build search criteria
	criteria := imap.NewSearchCriteria()

	if filters.Since != nil {
		criteria.Since = *filters.Since
	} else if filters.LastDays > 0 {
		since := time.Now().AddDate(0, 0, -filters.LastDays)
		criteria.Since = since
	}

	if filters.Before != nil {
		criteria.Before = *filters.Before
	}

	if filters.UnreadOnly {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	// Search for messages
	uids, err := c.client.UidSearch(criteria)
	if err != nil {
		return 0, fmt.Errorf("failed to search emails: %w", err)
	}

	return len(uids), nil
}

// MarkRead marks an email as read or unread
func (c *Client) MarkRead(ctx context.Context, folder, emailID string, read bool) error {
	// Select the mailbox
	if _, err := c.client.Select(folder, false); err != nil {
		return fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Parse UID
	var uid uint32
	if _, err := fmt.Sscanf(emailID, "%d", &uid); err != nil {
		return fmt.Errorf("invalid email ID format: %w", err)
	}

	// Create sequence set
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// Store flags
	var item imap.StoreItem
	if read {
		item = imap.FormatFlagsOp(imap.AddFlags, true)
	} else {
		item = imap.FormatFlagsOp(imap.RemoveFlags, true)
	}
	
	flags := []interface{}{imap.SeenFlag}
	if err := c.client.UidStore(seqSet, item, flags, nil); err != nil {
		return fmt.Errorf("failed to mark email: %w", err)
	}

	return nil
}

// MoveEmail moves an email from one folder to another
func (c *Client) MoveEmail(ctx context.Context, fromFolder, toFolder, emailID string) error {
	// Select the source mailbox
	if _, err := c.client.Select(fromFolder, false); err != nil {
		return fmt.Errorf("failed to select folder %s: %w", fromFolder, err)
	}

	// Parse UID
	var uid uint32
	if _, err := fmt.Sscanf(emailID, "%d", &uid); err != nil {
		return fmt.Errorf("invalid email ID format: %w", err)
	}

	// Create sequence set
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// Try to use MOVE command (if supported)
	// Otherwise fall back to COPY + DELETE
	if err := c.client.UidMove(seqSet, toFolder); err != nil {
		// Fallback: Copy then mark as deleted
		if err := c.client.UidCopy(seqSet, toFolder); err != nil {
			return fmt.Errorf("failed to copy email: %w", err)
		}
		
		// Mark as deleted
		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}
		if err := c.client.UidStore(seqSet, item, flags, nil); err != nil {
			return fmt.Errorf("failed to mark email as deleted: %w", err)
		}
		
		// Expunge to remove it
		if err := c.client.Expunge(nil); err != nil {
			return fmt.Errorf("failed to expunge: %w", err)
		}
	}

	return nil
}

// DeleteEmail deletes an email (moves to trash or permanently deletes)
func (c *Client) DeleteEmail(ctx context.Context, folder, emailID string, permanent bool) error {
	if permanent {
		// Select the mailbox
		if _, err := c.client.Select(folder, false); err != nil {
			return fmt.Errorf("failed to select folder %s: %w", folder, err)
		}

		// Parse UID
		var uid uint32
		if _, err := fmt.Sscanf(emailID, "%d", &uid); err != nil {
			return fmt.Errorf("invalid email ID format: %w", err)
		}

		// Create sequence set
		seqSet := new(imap.SeqSet)
		seqSet.AddNum(uid)

		// Mark as deleted
		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}
		if err := c.client.UidStore(seqSet, item, flags, nil); err != nil {
			return fmt.Errorf("failed to mark email as deleted: %w", err)
		}

		// Expunge to permanently delete
		if err := c.client.Expunge(nil); err != nil {
			return fmt.Errorf("failed to expunge: %w", err)
		}
	} else {
		// Move to Trash folder
		trashFolder := "Deleted Messages"
		if err := c.MoveEmail(ctx, folder, trashFolder, emailID); err != nil {
			// Try alternate trash folder name
			trashFolder = "Trash"
			if err := c.MoveEmail(ctx, folder, trashFolder, emailID); err != nil {
				return fmt.Errorf("failed to move to trash: %w", err)
			}
		}
	}

	return nil
}

// parseMessageData parses IMAP message data into Email struct
func (c *Client) parseMessageData(msg *imap.Message, fetchBody bool) *Email {
	if msg.Envelope == nil {
		return nil
	}

	// Check if message has Seen flag
	unread := true
	for _, flag := range msg.Flags {
		if flag == imap.SeenFlag {
			unread = false
			break
		}
	}

	email := &Email{
		ID:      fmt.Sprintf("%d", msg.Uid),
		Subject: msg.Envelope.Subject,
		Date:    msg.Envelope.Date,
		Unread:  unread,
	}

	// Parse From
	if len(msg.Envelope.From) > 0 {
		email.From = formatAddress(msg.Envelope.From[0])
	}

	// Parse To
	email.To = make([]string, 0, len(msg.Envelope.To))
	for _, addr := range msg.Envelope.To {
		email.To = append(email.To, formatAddress(addr))
	}

	// Parse CC
	email.CC = make([]string, 0, len(msg.Envelope.Cc))
	for _, addr := range msg.Envelope.Cc {
		email.CC = append(email.CC, formatAddress(addr))
	}

	// Parse BCC
	email.BCC = make([]string, 0, len(msg.Envelope.Bcc))
	for _, addr := range msg.Envelope.Bcc {
		email.BCC = append(email.BCC, formatAddress(addr))
	}

	// Store Message-ID
	email.MessageID = msg.Envelope.MessageId

	// Parse In-Reply-To and References
	if msg.Envelope.InReplyTo != "" {
		email.References = append(email.References, msg.Envelope.InReplyTo)
	}

	// Parse body if requested
	if fetchBody {
		for _, literal := range msg.Body {
			c.parseEmailBody(email, literal)
			break
		}
	} else {
		// Create snippet from subject for preview
		if len(email.Subject) > 200 {
			email.Snippet = email.Subject[:197] + "..."
		} else {
			email.Snippet = email.Subject
		}
	}

	return email
}

// parseEmailBody parses the email body and attachments
func (c *Client) parseEmailBody(email *Email, bodyLiteral imap.Literal) {
	if bodyLiteral == nil {
		return
	}
	
	msg, err := mail.ReadMessage(bodyLiteral)
	if err != nil {
		return
	}

	// Parse the message using go-message
	mr, err := message.CreateReader(msg.Body)
	if err != nil {
		return
	}

	// Process message parts
	c.processMessagePart(email, mr)

	// Create snippet from plain text body
	if email.BodyPlain != "" {
		snippet := strings.TrimSpace(email.BodyPlain)
		if len(snippet) > 200 {
			email.Snippet = snippet[:197] + "..."
		} else {
			email.Snippet = snippet
		}
	} else if email.BodyHTML != "" {
		// Use subject as snippet if no plain text
		snippet := email.Subject
		if len(snippet) > 200 {
			email.Snippet = snippet[:197] + "..."
		} else {
			email.Snippet = snippet
		}
	}
}

// processMessagePart recursively processes message parts
func (c *Client) processMessagePart(email *Email, mr *message.Reader) {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}

		switch h := part.Header.(type) {
		case *message.InlineHeader:
			contentType, _, _ := h.ContentType()
			body, _ := io.ReadAll(part.Body)

			if strings.HasPrefix(contentType, "text/plain") {
				email.BodyPlain = string(body)
			} else if strings.HasPrefix(contentType, "text/html") {
				email.BodyHTML = string(body)
			}

		case *message.AttachmentHeader:
			filename, _ := h.Filename()
			if filename != "" {
				// Count size without reading full content
				size, _ := io.Copy(io.Discard, part.Body)
				email.Attachments = append(email.Attachments, Attachment{
					Filename: filename,
					Size:     size,
				})
			}

		}
	}
}

// formatAddress formats an IMAP address into a string
func formatAddress(addr *imap.Address) string {
	if addr.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
	}
	return fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
}

// GetUsername returns the authenticated username
func (c *Client) GetUsername() string {
	return c.username
}
