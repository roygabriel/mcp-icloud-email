# Audit Findings & Improvement Plan

Audited against `docs/mcp-server-audit-prompt.md`. Every Go file was read in full.

---

## P0 - Critical Bugs & Race Conditions

### F1. Shared IMAP connection without synchronization
**Files**: `imap/client.go` (entire file), `main.go:25`
**Impact**: Data corruption, panics under concurrent MCP requests

The single `*client.Client` is shared across all 14 tool handlers. The MCP stdio server dispatches requests concurrently. IMAP is a sequential protocol per connection - interleaving `Select()` + `UidSearch()` + `UidFetch()` from different goroutines will corrupt the protocol state. Example: handler A calls `Select("INBOX")`, handler B calls `Select("Sent")` before A's `UidSearch()` runs - A now searches the wrong folder.

**Fix**: Add a `sync.Mutex` to serialize all IMAP operations on the client, or use a connection pool.

### F2. Message-ID generated with `time.Now().UnixNano()` - collision risk
**Files**: `imap/client.go:649`, `smtp/client.go:75`
**Impact**: Duplicate Message-IDs if two operations occur within same nanosecond

Both `SaveDraft()` and `SendEmail()` generate Message-IDs using `time.Now().UnixNano()`. Under concurrent requests or clock jitter, these can collide. The `google/uuid` package is already an indirect dependency.

**Fix**: Replace with `uuid.New().String()` from `github.com/google/uuid`.

### F3. Context parameter accepted but never used
**Files**: Every method in `imap/client.go`, `smtp/client.go`
**Impact**: Hung requests block the server forever; no cancellation support

Every IMAP/SMTP method accepts `ctx context.Context` but never checks `ctx.Done()`, never wraps operations with `context.WithTimeout()`, and the underlying `go-imap` library calls don't accept contexts. A network stall permanently blocks the handler goroutine.

**Fix**: At minimum, add handler-level timeouts via `context.WithTimeout()`. For IMAP, use the client's `Timeout` field. For SMTP, use `net.DialTimeout` or a deadline on the connection.

### F4. Goroutine leak potential in IMAP fetch operations
**Files**: `imap/client.go:118-120` (ListFolders), `189-191` (SearchEmails), `229-231` (GetEmail), `703-705` and `726-728` (GetAttachment)
**Impact**: Goroutine accumulation on cancelled/timed-out requests

Goroutines are spawned with `go func() { done <- c.client.List/UidFetch(...) }()` but there's no cancellation path. If the parent context is cancelled, the goroutine runs to completion or blocks indefinitely.

**Fix**: Coupled with F3 - once context support is added, ensure goroutines respect cancellation.

### F5. Errors silently swallowed during email parsing
**Files**: `imap/client.go:482-483`, `488-489`, `521-523`, `527-528`
**Impact**: Silent data loss - emails appear to have no body/attachments when parsing fails

`parseEmailBody()` and `processMessagePart()` silently return on errors. A malformed email silently loses its body content. At minimum these should be logged.

**Fix**: Log parsing errors at WARN level (after F8 structured logging is added).

---

## P0 - No Test Coverage

### F6. Zero test files
**Files**: None exist
**Impact**: Highest-leverage improvement possible. No way to verify correctness or catch regressions.

No `*_test.go` files in any package. All changes are deployed blind.

**Fix**: Extract service interfaces (F16), create mock clients, write table-driven tests for every handler. Must pass `go test -race ./...`.

### F7. Concrete types prevent testability
**Files**: All `tools/*.go` handler functions accept `*imap.Client` / `*smtp.Client`
**Impact**: Cannot test handlers without a live IMAP/SMTP server

Every handler function signature uses concrete types. Example: `SearchEmailsHandler(client *imap.Client)`. No way to inject a mock.

**Fix**: Extract `EmailService` and `SMTPService` interfaces (see F16), update handler signatures.

---

## P0 - Structured Logging

### F8. No structured logging
**Files**: `main.go:21,27,35,309-312,316`
**Impact**: No way to debug production issues; `fmt.Fprintf(os.Stderr, ...)` is unstructured

Uses `log.Fatalf` for fatal errors and `fmt.Fprintf(os.Stderr, ...)` for startup messages. No contextual fields, no log levels, no request correlation.

**Fix**: Replace with `log/slog` using `slog.NewJSONHandler(os.Stderr, ...)`. Add `LOG_LEVEL` env var. Add request ID, tool name, and duration fields.

---

## P0 - Graceful Shutdown

### F9. No signal handling or graceful shutdown
**Files**: `main.go:315`
**Impact**: Unclean IMAP logout, potential data loss on in-flight operations

`server.ServeStdio(s)` blocks without a cancellable context. No SIGTERM/SIGINT handler. The deferred `imapClient.Close()` only runs if the process exits normally through main().

**Fix**: Use `server.NewStdioServer(s).Listen(ctx, os.Stdin, os.Stdout)` with a signal-cancellable context. Register `os.Signal` handlers for SIGTERM/SIGINT.

---

## P1 - Input Validation & Sanitization

### F10. Path traversal in `get_attachment` save_path
**Files**: `tools/get_attachment.go:39,55-65`
**Impact**: Arbitrary file write to any location on disk

The `save_path` parameter only checks if the parent directory exists. An agent could pass `save_path: "/etc/cron.d/malicious"` or `"../../.ssh/authorized_keys"`. No traversal validation, no symlink check, no allowlist.

**Fix**: Reject paths containing `..`, null bytes, or characters outside a safe set. Consider requiring paths under a configurable base directory.

### F11. No folder name sanitization
**Files**: `tools/folder.go:18-29`, `imap/client.go:880-893`
**Impact**: IMAP command injection via crafted folder names

`create_folder` concatenates `parent + "/" + name` without validating for `..`, null bytes (`\x00`), or IMAP-special characters. Same issue in `delete_folder`.

**Fix**: Add `ValidateFolderName()` helper rejecting `..`, null bytes, newlines, and IMAP list wildcards (`*`, `%`).

### F12. No input size limits
**Files**: `tools/send_email.go`, `tools/draft_email.go`, `tools/reply_email.go`
**Impact**: Resource exhaustion via extremely large email bodies

No maximum length validation on `body`, `subject`, or attachment counts. An agent could send a multi-gigabyte body.

**Fix**: Add reasonable upper bounds (e.g., body < 10MB, subject < 998 chars per RFC 2822).

---

## P1 - Retry Logic & Resilience

### F13. No IMAP reconnection logic
**Files**: `imap/client.go:79-103`, `main.go:25-29`
**Impact**: Single connection created at startup; if it drops, ALL subsequent requests fail permanently

The IMAP connection is established once in `main()`. Network hiccup, server restart, or idle timeout kills the connection permanently. Every tool call after that returns errors.

**Fix**: Add connection health check and automatic reconnection. Wrap the client with a reconnecting decorator that detects dead connections and re-establishes them.

### F14. No handler-level timeouts
**Files**: All `tools/*.go` handlers
**Impact**: Hung IMAP operations block the handler goroutine forever

No `context.WithTimeout()` wrapping any handler. Related to F3 but actionable independently via MCP middleware.

**Fix**: Use `server.WithToolHandlerMiddleware()` to wrap all handlers with a default timeout (e.g., 30s).

---

## P1 - Configuration Hardening

### F15. CI Go version mismatch
**Files**: `go.mod:3` (`go 1.25.1`), `.github/workflows/release.yml:47` (`go-version: '1.21'`)
**Impact**: CI builds with a different Go version than development; may not compile

The go.mod declares `go 1.25.1` but CI uses `go-version: '1.21'`. This will fail if any 1.25+ features are used.

**Fix**: Change CI to `go-version-file: 'go.mod'`.

---

## P1 - Tool Schema Quality

### F16. Missing tool annotations on all 14 tools
**Files**: `main.go:50-306`
**Impact**: AI agents cannot assess tool safety; no read-only/destructive/idempotent hints

No tool uses `WithReadOnlyHintAnnotation`, `WithDestructiveHintAnnotation`, or `WithIdempotentHintAnnotation`. Agents can't distinguish safe reads from destructive deletes.

**Fixes needed**:
| Tool | ReadOnly | Destructive | Idempotent |
|------|----------|-------------|------------|
| search_emails | true | false | true |
| get_email | true | false | true |
| list_folders | true | false | true |
| count_emails | true | false | true |
| get_attachment | true | false | true |
| send_email | false | false | false |
| reply_email | false | false | false |
| draft_email | false | false | false |
| mark_read | false | false | true |
| move_email | false | false | true |
| flag_email | false | false | true |
| delete_email | false | true | true |
| create_folder | false | false | false |
| delete_folder | false | true | false |

### F17. Vague tool descriptions without workflow guidance
**Files**: `main.go:50-306`
**Impact**: AI agents guess at parameter values and call order

Examples:
- `search_emails`: Doesn't mention "Use list_folders first to discover valid folder names"
- `get_email`: Doesn't say "Returns full body, headers, and attachment metadata. Use email_id from search_emails."
- `delete_email`: Doesn't explain "Moves to 'Deleted Messages' by default. Set permanent=true for immediate removal."
- `send_email`: Doesn't mention what the response contains
- `get_attachment`: Doesn't say "Use get_email first to see available attachment filenames"

### F18. No schema constraints (Enum, Min/Max, MinLength, Default)
**Files**: `main.go:50-306`
**Impact**: Agents guess at valid values; no programmatic validation

Missing constraints:
- `limit`: No `Min(1)`, `Max(200)`, `DefaultNumber(50)`
- `last_days`: No `Min(1)`, `DefaultNumber(30)`
- `flag`: No `Enum("follow-up", "important", "deadline", "none")`
- `color`: No `Enum("red", "orange", "yellow", "green", "blue", "purple")`
- `email_id`: No `MinLength(1)` on any tool
- `folder`: No `DefaultString("INBOX")` on any tool
- `since`/`before`: No `Pattern` for RFC 3339 format
- Inconsistent format naming: says "ISO 8601" in description but parses RFC 3339

---

## P2 - Interface & Architecture

### F19. No service interfaces
**Files**: `imap/client.go`, `smtp/client.go`, all `tools/*.go`
**Impact**: Cannot test, cannot add decorators, cannot support multi-account

All handlers depend on concrete `*imap.Client` and `*smtp.Client`. Blocks testing (F6), retry decoration (F13), rate limiting, metrics.

**Fix**: Extract `EmailReader`, `EmailWriter`, `EmailSender` interfaces (or a single `EmailService`).

### F20. Duplicated argument-parsing logic
**Files**: `tools/send_email.go:30-48`, `tools/draft_email.go:31-49` (identical to/cc/bcc parsing)
**Impact**: Maintenance burden, inconsistency risk

The `to`, `cc`, `bcc` parsing + validation code is copy-pasted between `send_email.go` and `draft_email.go`.

**Fix**: Extract `parseAddressList(args, key)` helper.

---

## P2 - Build & CI

### F21. No Makefile
**Impact**: No standardized build commands

### F22. No linting configuration
**Impact**: No static analysis catching bugs before deployment

### F23. No Dockerfile
**Impact**: Cannot containerize

### F24. No vulnerability scanning
**Impact**: No `govulncheck`, no Dependabot/Renovate

### F25. No version injection
**Files**: `main.go:44` (hardcoded `"1.0.0"`)
**Impact**: Cannot determine which build is running in production

---

## P3 - Advanced Features

### F26. No pagination (offset) on search
### F27. No multi-account support
### F28. No connection pooling configuration
### F29. No mTLS support

---

## Implementation Plan (Dependency-Ordered Phases)

### Phase 1: Foundation (blocks everything else)
1. **F1** - Add `sync.Mutex` to IMAP client (or connection pool)
2. **F2** - Replace `time.Now().UnixNano()` with `uuid.New().String()`
3. **F8** - Add `log/slog` structured logging
4. **F9** - Add signal handling and graceful shutdown
5. **F15** - Fix CI Go version to use `go-version-file: 'go.mod'`

### Phase 2: Interfaces & Testability (blocks tests)
6. **F19** - Extract service interfaces (`EmailService`, `SMTPService`)
7. **F7** - Update handler signatures to use interfaces
8. **F20** - Extract shared argument-parsing helpers

### Phase 3: Test Coverage
9. **F6** - Create mock clients implementing interfaces
10. Write table-driven tests for all 14 handlers
11. Write unit tests for imap/smtp client methods
12. Verify all tests pass with `go test -race ./...`

### Phase 4: Input Validation & Resilience
13. **F10** - Add path traversal validation to `get_attachment` save_path
14. **F11** - Add folder name sanitization
15. **F12** - Add input size limits
16. **F3/F14** - Add handler-level timeouts via middleware
17. **F13** - Add IMAP reconnection logic
18. **F5** - Log parsing errors (now possible with slog from Phase 1)

### Phase 5: Tool Schema Quality
19. **F16** - Add tool annotations (read-only, destructive, idempotent)
20. **F17** - Rewrite tool descriptions with workflow guidance
21. **F18** - Add schema constraints (Enum, Min/Max, MinLength, Default)

### Phase 6: Build & CI Hardening
22. **F21** - Add Makefile
23. **F22** - Add `.golangci.yml`
24. **F23** - Add Dockerfile
25. **F24** - Add govulncheck + Dependabot
26. **F25** - Add version injection via ldflags

### Phase 7: Observability (optional, high value)
27. Add request ID middleware
28. Add `/healthz` + `/readyz` endpoints
29. Add Prometheus metrics

### Phase 8: Advanced (P3, if desired)
30. **F26** - Add pagination offset to search
31. **F27** - Multi-account support
