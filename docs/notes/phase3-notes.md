# Phase 3: Test Coverage - Working Notes

## Changes Made

### Mock clients (tools/mock_test.go)
- `MockEmailService` implements `EmailService` (all 12 IMAP methods)
- `MockEmailSender` implements `EmailSender` (2 SMTP methods)
- Both support per-method error injection via `Err` field
- Both track last call parameters (LastFolder, LastEmailID, etc.) and CallCount

### Test suite (tools/handlers_test.go)
Table-driven tests for all 14 handlers plus shared helpers:

| Test | Cases |
|------|-------|
| TestListFoldersHandler | 2 (happy, error) |
| TestGetEmailHandler | 5 (happy, folder, missing id, empty id, error) |
| TestSearchEmailsHandler | 7 (defaults, query+folder, limit cap, since override, invalid since, invalid before, error) |
| TestCountEmailsHandler | 3 (defaults, filters, error) |
| TestMarkReadHandler | 4 (read default, unread, missing id, error) |
| TestMoveEmailHandler | 5 (happy, from_folder, missing id, missing to_folder, error) |
| TestDeleteEmailHandler | 4 (trash default, permanent, missing id, error) |
| TestFlagEmailHandler | 8 (follow-up, color, none, missing id, missing flag, invalid flag, invalid color, error) |
| TestSendEmailHandler | 8 (happy, cc/bcc, array to, missing to/subject/body, invalid email, error) |
| TestReplyEmailHandler | 6 (happy, reply-all+html, missing id/body, IMAP error, SMTP error) |
| TestDraftEmailHandler | 6 (happy, reply_to_id, missing to/subject/body, error) |
| TestGetAttachmentHandler | 4 (base64, missing id, missing filename, error) |
| TestCreateFolderHandler | 4 (happy, parent, missing name, error) |
| TestDeleteFolderHandler | 4 (empty, force, non-empty structured, missing name) |
| TestParseAddressList | 6 (string, array, missing, nil, invalid, wrong type) |
| TestRequireAddressList | 2 (missing, success) |

**Total: 78 test cases across 16 test functions**

## Verification
- `go test -race ./...` passes
- All tests are table-driven with descriptive names
- Test helper functions: `req()`, `resultJSON()`, `resultErrText()`
