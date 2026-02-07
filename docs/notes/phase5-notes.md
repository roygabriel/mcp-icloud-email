# Phase 5: Tool Schema Quality - Working Notes

## Changes Made

### F16: Tool annotations on all 14 tools
Every tool now has:
- `WithReadOnlyHintAnnotation` - true for search/get/list/count, false for mutating tools
- `WithDestructiveHintAnnotation` - true only for delete_email and delete_folder
- `WithIdempotentHintAnnotation` - true for reads and updates, false for creates and sends

### F17: Rewritten descriptions with workflow guidance
- Every tool description explains what it does, what it returns, and what to call first
- Cross-references: "Use list_folders first to discover valid folder names"
- Cross-references: "Use search_emails first to find email IDs"
- Cross-references: "Use get_email first to see available attachment filenames"
- Side effects documented: "Calling twice will send duplicate emails"
- Destructive warnings: "This cannot be undone"

### F18: Schema constraints
Applied where applicable:
- `Enum()` on flag ("follow-up", "important", "deadline", "none") and color ("red"..."purple")
- `Min(1)`, `Max(200)`, `DefaultNumber(50)` on limit
- `Min(1)`, `DefaultNumber(30)` on last_days
- `MinLength(1)` on all required string fields (email_id, to, subject, body, filename, name, to_folder)
- `DefaultString("INBOX")` on all folder parameters
- `DefaultBool(false)` on html, unread_only, permanent, force, reply_all
- `DefaultBool(true)` on read (mark_read)
- Consistent "RFC 3339 format" (not "ISO 8601") with example values

## Verification
- `go build ./...` passes
- `go vet ./...` passes
- `go test -race ./...` passes
