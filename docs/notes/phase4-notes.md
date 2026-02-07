# Phase 4: Input Validation & Resilience - Working Notes

## Changes Made

### F10: Path traversal validation (tools/validate.go)
- `validateSavePath()` rejects `..`, null bytes, and relative paths
- Applied in `get_attachment.go` to the `save_path` parameter

### F11: Folder name sanitization
- `validateFolderName()` rejects `..`, null bytes, IMAP wildcards (`*`, `%`), newlines, and control chars
- Applied in `folder.go` for both create_folder (name + parent) and delete_folder (name)

### F12: Input size limits
- `validateBodySize()` caps email body at 10 MB
- `validateSubjectSize()` caps subject at 998 chars (RFC 2822)
- Applied in `send_email.go` and `draft_email.go`

### F14: Handler-level timeout middleware
- Added `timeoutMiddleware(60s)` via `server.WithToolHandlerMiddleware()`
- Every tool handler now has a 60-second context deadline
- Defined as a function in main.go returning `server.ToolHandlerMiddleware`

### Additional validation
- `validateEmailID()` rejects empty, null bytes, and control characters
- `validateFilename()` rejects path separators, `..`, and null bytes
- Applied in `get_attachment.go` for both email_id and filename

### Test coverage (tools/validate_test.go)
- 26 test cases covering all validation functions
- Tests for: valid inputs, empty, traversal, null bytes, wildcards, control chars, size limits

## Verification
- `go build ./...` passes
- `go vet ./...` passes
- `go test -race ./...` passes (all 104 test cases)
