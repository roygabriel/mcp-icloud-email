# Phase 1: Foundation - Working Notes

## Changes Made

### F1: sync.Mutex on IMAP client
- Added `sync.Mutex` field `mu` to `imap.Client` struct
- Every public method now does `c.mu.Lock(); defer c.mu.Unlock()`
- Created internal (unlocked) versions for methods called by other locked methods:
  - `listFolders()` (called by `SaveDraft`)
  - `getEmail()` (called by `SaveDraft` for reply drafts)
  - `countEmails()` (called by `DeleteFolder`)
  - `moveEmail()` (called by `DeleteEmail`)
- This prevents deadlock from nested locking

### F2: UUID for Message-ID
- Replaced `time.Now().UnixNano()` with `uuid.New().String()` in:
  - `imap/client.go` SaveDraft
  - `smtp/client.go` SendEmail
- Promoted `github.com/google/uuid` from indirect to direct dependency

### F5: Log parse errors (partial - full fix with slog)
- Added `slog.Warn()` calls in `parseEmailBody()` and `processMessagePart()`
- Previously: errors silently returned, email appeared to have no body
- Now: parse failures are logged at WARN level

### F8: Structured logging (slog)
- Replaced all `log.Fatalf` / `fmt.Fprintf(os.Stderr, ...)` with `slog`
- Added `LOG_LEVEL` env var support (DEBUG, INFO, WARN, ERROR)
- JSON handler writes to stderr (keeps stdout clean for MCP JSON-RPC)
- Added `version` var (set via ldflags at build time, defaults to "dev")

### F9: Signal handling + graceful shutdown
- Added SIGINT/SIGTERM handler that cancels root context
- Replaced `server.ServeStdio(s)` with `server.NewStdioServer(s).Listen(ctx, ...)`
- Context cancellation propagates to stdio listener for clean shutdown

### F15: CI Go version fix
- Changed `.github/workflows/release.yml` from `go-version: '1.21'` to `go-version-file: 'go.mod'`

## Verification
- `go build ./...` passes
- `go vet ./...` passes
- `go mod tidy` clean
