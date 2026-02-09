package imap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	goiMap "github.com/emersion/go-imap"
)

// --- MockBackend ---

type MockBackend struct {
	// Configurable return values
	ListMailboxes []*goiMap.MailboxInfo
	ListErr       error

	SelectStatus *goiMap.MailboxStatus
	SelectErr    error

	UidSearchUIDs []uint32
	UidSearchErr  error

	UidFetchMessages []*goiMap.Message
	UidFetchErr      error

	// Multi-call support: each UidFetch call pops from this slice.
	// When empty, falls back to UidFetchMessages/UidFetchErr.
	UidFetchCallResults []struct {
		Messages []*goiMap.Message
		Err      error
	}

	UidStoreErr error
	UidMoveErr  error
	UidCopyErr  error
	ExpungeErr  error
	AppendErr   error
	CreateErr   error
	DeleteErr   error
	LogoutErr   error

	// Call tracking
	ListCalls      int
	SelectCalls    int
	UidSearchCalls int
	UidFetchCalls  int
	UidStoreCalls  int
	UidMoveCalls   int
	UidCopyCalls   int
	ExpungeCalls   int
	AppendCalls    int
	CreateCalls    int
	DeleteCalls    int
	LogoutCalls    int

	// Captured arguments
	LastSelectName string
	LastCreateName string
	LastDeleteName string
	LastMoveDest   string
	LastCopyDest   string
	LastAppendMbox string
}

func (m *MockBackend) List(ref, name string, ch chan *goiMap.MailboxInfo) error {
	m.ListCalls++
	for _, mb := range m.ListMailboxes {
		ch <- mb
	}
	close(ch)
	return m.ListErr
}

func (m *MockBackend) Select(name string, readOnly bool) (*goiMap.MailboxStatus, error) {
	m.SelectCalls++
	m.LastSelectName = name
	return m.SelectStatus, m.SelectErr
}

func (m *MockBackend) UidSearch(criteria *goiMap.SearchCriteria) ([]uint32, error) {
	m.UidSearchCalls++
	return m.UidSearchUIDs, m.UidSearchErr
}

func (m *MockBackend) UidFetch(seqset *goiMap.SeqSet, items []goiMap.FetchItem, ch chan *goiMap.Message) error {
	m.UidFetchCalls++

	// Use per-call results if available
	if len(m.UidFetchCallResults) > 0 {
		result := m.UidFetchCallResults[0]
		m.UidFetchCallResults = m.UidFetchCallResults[1:]
		for _, msg := range result.Messages {
			ch <- msg
		}
		close(ch)
		return result.Err
	}

	for _, msg := range m.UidFetchMessages {
		ch <- msg
	}
	close(ch)
	return m.UidFetchErr
}

func (m *MockBackend) UidStore(seqset *goiMap.SeqSet, item goiMap.StoreItem, value interface{}, ch chan *goiMap.Message) error {
	m.UidStoreCalls++
	return m.UidStoreErr
}

func (m *MockBackend) UidMove(seqset *goiMap.SeqSet, dest string) error {
	m.UidMoveCalls++
	m.LastMoveDest = dest
	return m.UidMoveErr
}

func (m *MockBackend) UidCopy(seqset *goiMap.SeqSet, dest string) error {
	m.UidCopyCalls++
	m.LastCopyDest = dest
	return m.UidCopyErr
}

func (m *MockBackend) Expunge(ch chan uint32) error {
	m.ExpungeCalls++
	return m.ExpungeErr
}

func (m *MockBackend) Append(mbox string, flags []string, date time.Time, msg goiMap.Literal) error {
	m.AppendCalls++
	m.LastAppendMbox = mbox
	return m.AppendErr
}

func (m *MockBackend) Create(name string) error {
	m.CreateCalls++
	m.LastCreateName = name
	return m.CreateErr
}

func (m *MockBackend) Delete(name string) error {
	m.DeleteCalls++
	m.LastDeleteName = name
	return m.DeleteErr
}

func (m *MockBackend) Logout() error {
	m.LogoutCalls++
	return m.LogoutErr
}

// --- Helper ---

func newTestClient(mb *MockBackend) *Client {
	return NewClientWithBackend(mb, "test@icloud.com")
}

func makeEnvelope(subject, fromMailbox, fromHost, messageID string) *goiMap.Envelope {
	return &goiMap.Envelope{
		Subject:   subject,
		Date:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		From:      []*goiMap.Address{{MailboxName: fromMailbox, HostName: fromHost}},
		To:        []*goiMap.Address{{MailboxName: "test", HostName: "icloud.com"}},
		MessageId: messageID,
	}
}

func makeMessage(uid uint32, subject string, flags []string) *goiMap.Message {
	return &goiMap.Message{
		Uid:      uid,
		Envelope: makeEnvelope(subject, "sender", "example.com", "<msg-1@example.com>"),
		Flags:    flags,
	}
}

// --- Tests ---

func TestGetUsername(t *testing.T) {
	c := newTestClient(&MockBackend{})
	if got := c.GetUsername(); got != "test@icloud.com" {
		t.Errorf("GetUsername() = %q, want %q", got, "test@icloud.com")
	}
}

func TestClose(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mb := &MockBackend{}
		c := newTestClient(mb)
		if err := c.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if mb.LogoutCalls != 1 {
			t.Errorf("Logout called %d times, want 1", mb.LogoutCalls)
		}
	})

	t.Run("error", func(t *testing.T) {
		mb := &MockBackend{LogoutErr: errors.New("logout failed")}
		c := newTestClient(mb)
		if err := c.Close(); err == nil {
			t.Fatal("expected error from Close()")
		}
	})

	t.Run("nil backend", func(t *testing.T) {
		c := &Client{}
		if err := c.Close(); err != nil {
			t.Fatalf("Close() with nil backend error = %v", err)
		}
	})
}

func TestListFolders(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{
				{Name: "INBOX"},
				{Name: "Sent Messages"},
				{Name: "Drafts"},
			},
		}
		c := newTestClient(mb)

		folders, err := c.ListFolders(ctx)
		if err != nil {
			t.Fatalf("ListFolders() error = %v", err)
		}
		if len(folders) != 3 {
			t.Fatalf("got %d folders, want 3", len(folders))
		}
		if folders[0] != "INBOX" {
			t.Errorf("folders[0] = %q, want INBOX", folders[0])
		}
	})

	t.Run("empty", func(t *testing.T) {
		mb := &MockBackend{}
		c := newTestClient(mb)

		folders, err := c.ListFolders(ctx)
		if err != nil {
			t.Fatalf("ListFolders() error = %v", err)
		}
		if len(folders) != 0 {
			t.Fatalf("got %d folders, want 0", len(folders))
		}
	})

	t.Run("error", func(t *testing.T) {
		mb := &MockBackend{ListErr: errors.New("connection lost")}
		c := newTestClient(mb)

		_, err := c.ListFolders(ctx)
		if err == nil {
			t.Fatal("expected error from ListFolders()")
		}
	})
}

func TestSearchEmails(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("basic search", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100, 101, 102},
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Subj A", nil), makeMessage(101, "Subj B", nil), makeMessage(102, "Subj C", nil)},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{})
		if err != nil {
			t.Fatalf("SearchEmails() error = %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(emails) != 3 {
			t.Errorf("len(emails) = %d, want 3", len(emails))
		}
	})

	t.Run("with text query", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100},
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Match", nil)},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "important", EmailFilters{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if total != 1 || len(emails) != 1 {
			t.Errorf("total=%d, len=%d, want 1,1", total, len(emails))
		}
	})

	t.Run("with filters", func(t *testing.T) {
		since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		before := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100},
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Filtered", []string{goiMap.SeenFlag})},
		}
		c := newTestClient(mb)

		emails, _, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{
			Since:      &since,
			Before:     &before,
			UnreadOnly: true,
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(emails) != 1 {
			t.Errorf("len(emails) = %d, want 1", len(emails))
		}
		// Email has SeenFlag so Unread should be false
		if emails[0].Unread {
			t.Error("expected Unread=false for message with SeenFlag")
		}
	})

	t.Run("with last_days filter", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100},
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Recent", nil)},
		}
		c := newTestClient(mb)

		_, _, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{LastDays: 7})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("pagination offset", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100, 101, 102, 103, 104},
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "A", nil), makeMessage(101, "B", nil)},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{Offset: 3, Limit: 2})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(emails) != 2 {
			t.Errorf("len(emails) = %d, want 2", len(emails))
		}
	})

	t.Run("offset exceeds total", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{100, 101},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{Offset: 5})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
		if len(emails) != 0 {
			t.Errorf("len(emails) = %d, want 0", len(emails))
		}
	})

	t.Run("empty results", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if total != 0 || len(emails) != 0 {
			t.Errorf("total=%d, len=%d, want 0,0", total, len(emails))
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("no such folder")}
		c := newTestClient(mb)

		_, _, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("search error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidSearchErr: errors.New("search failed"),
		}
		c := newTestClient(mb)

		_, _, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{100},
			UidFetchErr:   errors.New("fetch failed"),
		}
		c := newTestClient(mb)

		_, _, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetEmail(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("success", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Test Email", nil)},
		}
		c := newTestClient(mb)

		email, err := c.GetEmail(ctx, "INBOX", "100")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if email.Subject != "Test Email" {
			t.Errorf("Subject = %q, want %q", email.Subject, "Test Email")
		}
		if email.ID != "100" {
			t.Errorf("ID = %q, want %q", email.ID, "100")
		}
	})

	t.Run("not found", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidFetchMessages: []*goiMap.Message{}, // no messages returned
		}
		c := newTestClient(mb)

		_, err := c.GetEmail(ctx, "INBOX", "999")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "email not found" {
			t.Errorf("error = %q, want 'email not found'", err.Error())
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		_, err := c.GetEmail(ctx, "INBOX", "abc")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		_, err := c.GetEmail(ctx, "BadFolder", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidFetchMessages: []*goiMap.Message{makeMessage(100, "Email", nil)},
			UidFetchErr:      errors.New("fetch failed"),
		}
		c := newTestClient(mb)

		_, err := c.GetEmail(ctx, "INBOX", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nil envelope", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchMessages: []*goiMap.Message{
				{Uid: 100, Envelope: nil},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetEmail(ctx, "INBOX", "100")
		if err == nil {
			t.Fatal("expected error for nil envelope")
		}
	})
}

func TestCountEmails(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 10}

	t.Run("success", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1, 2, 3, 4, 5},
		}
		c := newTestClient(mb)

		count, err := c.CountEmails(ctx, "INBOX", EmailFilters{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if count != 5 {
			t.Errorf("count = %d, want 5", count)
		}
	})

	t.Run("with filters", func(t *testing.T) {
		since := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1, 2},
		}
		c := newTestClient(mb)

		count, err := c.CountEmails(ctx, "INBOX", EmailFilters{Since: &since, UnreadOnly: true})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}
	})

	t.Run("with last_days", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1},
		}
		c := newTestClient(mb)

		count, err := c.CountEmails(ctx, "INBOX", EmailFilters{LastDays: 7})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("with before filter", func(t *testing.T) {
		before := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1},
		}
		c := newTestClient(mb)

		count, err := c.CountEmails(ctx, "INBOX", EmailFilters{Before: &before})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		_, err := c.CountEmails(ctx, "BadFolder", EmailFilters{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("search error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidSearchErr: errors.New("search failed"),
		}
		c := newTestClient(mb)

		_, err := c.CountEmails(ctx, "INBOX", EmailFilters{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMarkRead(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("mark as read", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.MarkRead(ctx, "INBOX", "100", true)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.UidStoreCalls != 1 {
			t.Errorf("UidStore called %d times, want 1", mb.UidStoreCalls)
		}
	})

	t.Run("mark as unread", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.MarkRead(ctx, "INBOX", "100", false)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.UidStoreCalls != 1 {
			t.Errorf("UidStore called %d times, want 1", mb.UidStoreCalls)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.MarkRead(ctx, "INBOX", "abc", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		err := c.MarkRead(ctx, "BadFolder", "100", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("store error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidStoreErr:  errors.New("store failed"),
		}
		c := newTestClient(mb)

		err := c.MarkRead(ctx, "INBOX", "100", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMoveEmail(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("move success", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "100")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.UidMoveCalls != 1 {
			t.Errorf("UidMove called %d times, want 1", mb.UidMoveCalls)
		}
		if mb.LastMoveDest != "Archive" {
			t.Errorf("move dest = %q, want %q", mb.LastMoveDest, "Archive")
		}
	})

	t.Run("fallback to copy+delete", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidMoveErr:   errors.New("MOVE not supported"),
		}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "100")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.UidCopyCalls != 1 {
			t.Errorf("UidCopy called %d times, want 1", mb.UidCopyCalls)
		}
		if mb.UidStoreCalls != 1 {
			t.Errorf("UidStore called %d times, want 1", mb.UidStoreCalls)
		}
		if mb.ExpungeCalls != 1 {
			t.Errorf("Expunge called %d times, want 1", mb.ExpungeCalls)
		}
	})

	t.Run("copy fallback error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidMoveErr:   errors.New("MOVE not supported"),
			UidCopyErr:   errors.New("copy failed"),
		}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fallback store error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidMoveErr:   errors.New("MOVE not supported"),
			UidStoreErr:  errors.New("store failed"),
		}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fallback expunge error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidMoveErr:   errors.New("MOVE not supported"),
			ExpungeErr:   errors.New("expunge failed"),
		}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "INBOX", "Archive", "abc")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		err := c.MoveEmail(ctx, "BadFolder", "Archive", "100")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDeleteEmail(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("permanent delete", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "100", true)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.UidStoreCalls != 1 {
			t.Errorf("UidStore called %d times, want 1", mb.UidStoreCalls)
		}
		if mb.ExpungeCalls != 1 {
			t.Errorf("Expunge called %d times, want 1", mb.ExpungeCalls)
		}
	})

	t.Run("permanent delete store error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidStoreErr:  errors.New("store failed"),
		}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "100", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("permanent delete expunge error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			ExpungeErr:   errors.New("expunge failed"),
		}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "100", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("permanent delete invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "abc", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("permanent delete select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("no folder")}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "BadFolder", "100", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("move to trash", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "100", false)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		// Should try "Deleted Messages" first via moveEmail
		if mb.UidMoveCalls < 1 {
			t.Error("expected at least one UidMove call")
		}
	})

	t.Run("move to trash fallback to Trash", func(t *testing.T) {
		callCount := 0
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidMoveErr: errors.New("MOVE not supported"),
			UidCopyErr: errors.New("no such folder"),
		}
		// Override UidCopy to fail first time (Deleted Messages) and succeed second (Trash)
		// Since the mock is simple, we need a more nuanced approach.
		// The moveEmail function will be called twice: first with "Deleted Messages", then "Trash".
		// Both will fail at UidMove, then fall to UidCopy which also fails.
		// So DeleteEmail will get an error from both moveEmail calls.
		// This tests the "failed to move to trash" path.
		_ = callCount
		c := newTestClient(mb)

		err := c.DeleteEmail(ctx, "INBOX", "100", false)
		if err == nil {
			t.Fatal("expected error when both trash folders fail")
		}
	})
}

func TestFlagEmail(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	tests := []struct {
		name     string
		flagType string
		color    string
		wantErr  bool
	}{
		{"follow-up", "follow-up", "", false},
		{"important", "important", "", false},
		{"deadline", "deadline", "", false},
		{"follow-up with red", "follow-up", "red", false},
		{"important with blue", "important", "blue", false},
		{"deadline with green", "deadline", "green", false},
		{"with orange", "follow-up", "orange", false},
		{"with yellow", "follow-up", "yellow", false},
		{"with purple", "follow-up", "purple", false},
		{"none clears all", "none", "", false},
		{"invalid flag type", "invalid", "", true},
		{"invalid color", "follow-up", "pink", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := &MockBackend{SelectStatus: okStatus}
			c := newTestClient(mb)

			err := c.FlagEmail(ctx, "INBOX", "100", tt.flagType, tt.color)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		err := c.FlagEmail(ctx, "BadFolder", "100", "follow-up", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		err := c.FlagEmail(ctx, "INBOX", "abc", "follow-up", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("store error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidStoreErr:  errors.New("store failed"),
		}
		c := newTestClient(mb)

		err := c.FlagEmail(ctx, "INBOX", "100", "follow-up", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCreateFolder(t *testing.T) {
	ctx := context.Background()

	t.Run("without parent", func(t *testing.T) {
		mb := &MockBackend{}
		c := newTestClient(mb)

		err := c.CreateFolder(ctx, "NewFolder", "")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.CreateCalls != 1 {
			t.Errorf("Create called %d times, want 1", mb.CreateCalls)
		}
		if mb.LastCreateName != "NewFolder" {
			t.Errorf("created folder = %q, want %q", mb.LastCreateName, "NewFolder")
		}
	})

	t.Run("with parent", func(t *testing.T) {
		mb := &MockBackend{}
		c := newTestClient(mb)

		err := c.CreateFolder(ctx, "Sub", "Parent")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.LastCreateName != "Parent/Sub" {
			t.Errorf("created folder = %q, want %q", mb.LastCreateName, "Parent/Sub")
		}
	})

	t.Run("error", func(t *testing.T) {
		mb := &MockBackend{CreateErr: errors.New("already exists")}
		c := newTestClient(mb)

		err := c.CreateFolder(ctx, "Existing", "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDeleteFolder(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 0}

	t.Run("empty folder", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{},
		}
		c := newTestClient(mb)

		wasEmpty, count, err := c.DeleteFolder(ctx, "EmptyFolder", false)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !wasEmpty {
			t.Error("expected wasEmpty=true")
		}
		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}
		if mb.DeleteCalls != 1 {
			t.Errorf("Delete called %d times, want 1", mb.DeleteCalls)
		}
	})

	t.Run("non-empty without force", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1, 2, 3},
		}
		c := newTestClient(mb)

		_, count, err := c.DeleteFolder(ctx, "Full", false)
		if err == nil {
			t.Fatal("expected error for non-empty folder without force")
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
		if mb.DeleteCalls != 0 {
			t.Errorf("Delete called %d times, want 0", mb.DeleteCalls)
		}
	})

	t.Run("non-empty with force", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{1, 2, 3},
		}
		c := newTestClient(mb)

		wasEmpty, count, err := c.DeleteFolder(ctx, "Full", true)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if wasEmpty {
			t.Error("expected wasEmpty=false")
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
		if mb.DeleteCalls != 1 {
			t.Errorf("Delete called %d times, want 1", mb.DeleteCalls)
		}
	})

	t.Run("access error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder not found")}
		c := newTestClient(mb)

		_, _, err := c.DeleteFolder(ctx, "Missing", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("delete error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:  okStatus,
			UidSearchUIDs: []uint32{},
			DeleteErr:     errors.New("delete failed"),
		}
		c := newTestClient(mb)

		_, _, err := c.DeleteFolder(ctx, "Folder", false)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSaveDraft(t *testing.T) {
	ctx := context.Background()
	draftStatus := &goiMap.MailboxStatus{Messages: 42}

	t.Run("basic draft", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "INBOX"}, {Name: "Drafts"}},
			SelectStatus:  draftStatus,
		}
		c := newTestClient(mb)

		id, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test Subject", "Hello", DraftOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if id != "42" {
			t.Errorf("id = %q, want %q", id, "42")
		}
		if mb.AppendCalls != 1 {
			t.Errorf("Append called %d times, want 1", mb.AppendCalls)
		}
		if mb.LastAppendMbox != "Drafts" {
			t.Errorf("appended to %q, want %q", mb.LastAppendMbox, "Drafts")
		}
	})

	t.Run("with CC and BCC", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
			SelectStatus:  draftStatus,
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{
			CC:  []string{"cc@example.com"},
			BCC: []string{"bcc@example.com"},
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("HTML draft", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
			SelectStatus:  draftStatus,
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "<p>Hello</p>", DraftOptions{HTML: true})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("fallback draft folder", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "INBOX"}}, // no Drafts folder found
			SelectStatus:  draftStatus,
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.LastAppendMbox != "Drafts" {
			t.Errorf("fallback appended to %q, want %q", mb.LastAppendMbox, "Drafts")
		}
	})

	t.Run("INBOX.Drafts folder", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "INBOX"}, {Name: "INBOX.Drafts"}},
			SelectStatus:  draftStatus,
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if mb.LastAppendMbox != "INBOX.Drafts" {
			t.Errorf("appended to %q, want %q", mb.LastAppendMbox, "INBOX.Drafts")
		}
	})

	t.Run("list folders error", func(t *testing.T) {
		mb := &MockBackend{ListErr: errors.New("list failed")}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("append error", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
			AppendErr:     errors.New("append failed"),
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("select after append error", func(t *testing.T) {
		selectCalls := 0
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
		}
		// We need SelectStatus to be nil (error) only on the second Select call.
		// Since our simple mock doesn't support that, we test the basic case where Select always errors.
		// Override: set SelectErr to fail
		_ = selectCalls
		mb.SelectErr = errors.New("select failed")
		// But listFolders doesn't call Select, so this won't affect it.
		// However, the Append call happens before the failing Select.
		// Since Select errors, the Append won't even be reached because
		// SaveDraft calls listFolders (uses List, not Select) then Append then Select.
		// Actually Append doesn't use Select. So append will succeed, then Select fails.
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("reply draft", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
			SelectStatus:  draftStatus,
			UidFetchMessages: []*goiMap.Message{
				{
					Uid: 50,
					Envelope: &goiMap.Envelope{
						Subject:   "Original Subject",
						MessageId: "<original@example.com>",
						From:      []*goiMap.Address{{MailboxName: "sender", HostName: "example.com"}},
						To:        []*goiMap.Address{{MailboxName: "test", HostName: "icloud.com"}},
						Date:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
					},
				},
			},
		}
		c := newTestClient(mb)

		id, err := c.SaveDraft(ctx, "test@icloud.com", []string{"sender@example.com"}, "Ignored", "Reply body", DraftOptions{
			ReplyToID: "50",
			Folder:    "INBOX",
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if id == "" {
			t.Error("expected non-empty draft ID")
		}
	})

	t.Run("reply draft default folder", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes: []*goiMap.MailboxInfo{{Name: "Drafts"}},
			SelectStatus:  draftStatus,
			UidFetchMessages: []*goiMap.Message{
				{
					Uid: 50,
					Envelope: &goiMap.Envelope{
						Subject:   "Re: Already replied",
						MessageId: "<msg@example.com>",
						From:      []*goiMap.Address{{MailboxName: "sender", HostName: "example.com"}},
						To:        []*goiMap.Address{{MailboxName: "test", HostName: "icloud.com"}},
						Date:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
					},
				},
			},
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"sender@example.com"}, "Ignored", "Reply body", DraftOptions{
			ReplyToID: "50",
			// Folder empty -> defaults to INBOX
		})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("reply draft get email error", func(t *testing.T) {
		mb := &MockBackend{
			ListMailboxes:    []*goiMap.MailboxInfo{{Name: "Drafts"}},
			SelectStatus:     draftStatus,
			UidFetchMessages: []*goiMap.Message{}, // no message -> email not found
		}
		c := newTestClient(mb)

		_, err := c.SaveDraft(ctx, "test@icloud.com", []string{"to@example.com"}, "Test", "Body", DraftOptions{
			ReplyToID: "999",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name string
		addr *goiMap.Address
		want string
	}{
		{
			name: "with personal name",
			addr: &goiMap.Address{PersonalName: "John Doe", MailboxName: "john", HostName: "example.com"},
			want: "John Doe <john@example.com>",
		},
		{
			name: "without personal name",
			addr: &goiMap.Address{MailboxName: "john", HostName: "example.com"},
			want: "john@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAddress(tt.addr)
			if got != tt.want {
				t.Errorf("formatAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMessageData(t *testing.T) {
	c := newTestClient(&MockBackend{})

	t.Run("nil envelope", func(t *testing.T) {
		msg := &goiMap.Message{}
		result := c.parseMessageData(msg, false)
		if result != nil {
			t.Fatal("expected nil for nil envelope")
		}
	})

	t.Run("basic message without body", func(t *testing.T) {
		msg := &goiMap.Message{
			Uid: 123,
			Envelope: &goiMap.Envelope{
				Subject:   "Test Subject",
				Date:      time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				From:      []*goiMap.Address{{PersonalName: "Sender", MailboxName: "sender", HostName: "example.com"}},
				To:        []*goiMap.Address{{MailboxName: "test", HostName: "icloud.com"}},
				Cc:        []*goiMap.Address{{MailboxName: "cc", HostName: "example.com"}},
				Bcc:       []*goiMap.Address{{MailboxName: "bcc", HostName: "example.com"}},
				MessageId: "<msg-123@example.com>",
				InReplyTo: "<parent@example.com>",
			},
			Flags: []string{},
		}

		email := c.parseMessageData(msg, false)
		if email == nil {
			t.Fatal("expected non-nil email")
		}
		if email.ID != "123" {
			t.Errorf("ID = %q, want %q", email.ID, "123")
		}
		if email.Subject != "Test Subject" {
			t.Errorf("Subject = %q", email.Subject)
		}
		if email.From != "Sender <sender@example.com>" {
			t.Errorf("From = %q", email.From)
		}
		if len(email.To) != 1 || email.To[0] != "test@icloud.com" {
			t.Errorf("To = %v", email.To)
		}
		if len(email.CC) != 1 {
			t.Errorf("CC = %v", email.CC)
		}
		if len(email.BCC) != 1 {
			t.Errorf("BCC = %v", email.BCC)
		}
		if email.MessageID != "<msg-123@example.com>" {
			t.Errorf("MessageID = %q", email.MessageID)
		}
		if len(email.References) != 1 || email.References[0] != "<parent@example.com>" {
			t.Errorf("References = %v", email.References)
		}
		if !email.Unread {
			t.Error("expected Unread=true (no SeenFlag)")
		}
		if email.Snippet != "Test Subject" {
			t.Errorf("Snippet = %q, want %q", email.Snippet, "Test Subject")
		}
	})

	t.Run("seen flag", func(t *testing.T) {
		msg := makeMessage(1, "Read Email", []string{goiMap.SeenFlag})
		email := c.parseMessageData(msg, false)
		if email.Unread {
			t.Error("expected Unread=false for message with SeenFlag")
		}
	})

	t.Run("long subject snippet truncation", func(t *testing.T) {
		longSubject := ""
		for i := 0; i < 210; i++ {
			longSubject += "a"
		}
		msg := makeMessage(1, longSubject, nil)
		email := c.parseMessageData(msg, false)
		if len(email.Snippet) != 200 {
			t.Errorf("Snippet length = %d, want 200", len(email.Snippet))
		}
	})

	t.Run("no from address", func(t *testing.T) {
		msg := &goiMap.Message{
			Uid: 1,
			Envelope: &goiMap.Envelope{
				Subject: "No From",
				Date:    time.Now(),
				From:    []*goiMap.Address{},
				To:      []*goiMap.Address{},
			},
		}
		email := c.parseMessageData(msg, false)
		if email.From != "" {
			t.Errorf("From = %q, want empty", email.From)
		}
	})
}

func TestGetAttachment(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 5}

	t.Run("email not found first fetch", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{}, Err: nil}, // first fetch: no messages
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "email not found" {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("select error", func(t *testing.T) {
		mb := &MockBackend{SelectErr: errors.New("folder error")}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "BadFolder", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		mb := &MockBackend{SelectStatus: okStatus}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "abc", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("fetch structure error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: errors.New("fetch failed")},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("email not found second fetch", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: nil},  // first: BODYSTRUCTURE
				{Messages: []*goiMap.Message{}, Err: nil},            // second: body - empty
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "email not found" {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("second fetch error", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: nil},
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: errors.New("body fetch failed")},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no body literal", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: nil},
				{Messages: []*goiMap.Message{{Uid: 100, Body: map[*goiMap.BodySectionName]goiMap.Literal{}}}, Err: nil},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
		if err.Error() != "failed to get message body" {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("malformed body", func(t *testing.T) {
		section := &goiMap.BodySectionName{}
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: nil},
				{Messages: []*goiMap.Message{{
					Uid: 100,
					Body: map[*goiMap.BodySectionName]goiMap.Literal{
						section: strings.NewReader("not a valid email"),
					},
				}}, Err: nil},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("parse error in body", func(t *testing.T) {
		section := &goiMap.BodySectionName{}
		// A valid email with headers but body content that go-message can't parse as MIME
		emailContent := "From: sender@example.com\r\n" +
			"To: test@icloud.com\r\n" +
			"Subject: Test\r\n" +
			"\r\n" +
			"plain body content\r\n"
		mb := &MockBackend{
			SelectStatus: okStatus,
			UidFetchCallResults: []struct {
				Messages []*goiMap.Message
				Err      error
			}{
				{Messages: []*goiMap.Message{{Uid: 100}}, Err: nil},
				{Messages: []*goiMap.Message{{
					Uid: 100,
					Body: map[*goiMap.BodySectionName]goiMap.Literal{
						section: strings.NewReader(emailContent),
					},
				}}, Err: nil},
			},
		}
		c := newTestClient(mb)

		_, err := c.GetAttachment(ctx, "INBOX", "100", "test.pdf")
		if err == nil {
			t.Fatal("expected error for unparseable body")
		}
	})
}

func TestSearchEmailsLimit(t *testing.T) {
	ctx := context.Background()
	okStatus := &goiMap.MailboxStatus{Messages: 10}

	t.Run("limit without offset", func(t *testing.T) {
		mb := &MockBackend{
			SelectStatus:     okStatus,
			UidSearchUIDs:    []uint32{100, 101, 102, 103, 104},
			UidFetchMessages: []*goiMap.Message{makeMessage(103, "D", nil), makeMessage(104, "E", nil)},
		}
		c := newTestClient(mb)

		emails, total, err := c.SearchEmails(ctx, "INBOX", "", EmailFilters{Limit: 2})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if total != 5 {
			t.Errorf("total = %d, want 5", total)
		}
		if len(emails) != 2 {
			t.Errorf("len(emails) = %d, want 2", len(emails))
		}
	})
}
