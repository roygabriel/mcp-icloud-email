# Phase 2: Interfaces & Testability - Working Notes

## Changes Made

### F19: Service interfaces (tools/interfaces.go)
- `EmailReader` - 5 read-only IMAP methods (ListFolders, SearchEmails, GetEmail, CountEmails, GetAttachment)
- `EmailWriter` - 7 mutating IMAP methods (MarkRead, MoveEmail, DeleteEmail, FlagEmail, SaveDraft, CreateFolder, DeleteFolder)
- `EmailService` - combines EmailReader + EmailWriter (the full `*imap.Client` satisfies this)
- `EmailSender` - 2 SMTP methods (SendEmail, ReplyToEmail)

### F7: Handler signature updates
Every handler now accepts an interface instead of a concrete type:
- Read-only handlers (search, get, list, count, get_attachment) -> `EmailReader`
- Write-only handlers (mark_read, move, delete, flag, create_folder, delete_folder) -> `EmailWriter`
- Draft handler -> `EmailWriter` (uses SaveDraft)
- Send handler -> `EmailSender`
- Reply handler -> `EmailReader` + `EmailSender`

### F20: Shared helpers (tools/helpers.go)
- `parseAddressList(args, key)` - extracts string or []interface{} to validated email slice
- `requireAddressList(args, key)` - same but errors if empty
- Replaces ~50 lines of duplicated to/cc/bcc parsing in send_email.go and draft_email.go

## Verification
- `go build ./...` passes
- `go vet ./...` passes
- `*imap.Client` satisfies `EmailService` (verified by compilation)
- `*smtp.Client` satisfies `EmailSender` (verified by compilation)
