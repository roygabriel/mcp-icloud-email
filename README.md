# iCloud Email MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that gives AI assistants full access to Apple iCloud Mail through IMAP and SMTP. Search, read, send, reply, organize, and manage your iCloud mailbox -- all from Claude or any MCP-compatible client.

Built with Go and the [mcp-go SDK](https://mcp-go.dev). Ships as a single static binary for Linux, macOS, and Windows.

## Features

- **Email Search & Retrieval** - Search emails with filters for date, read status, and text; retrieve full email content with attachments
- **Send & Reply** - Compose new emails or reply to threads with CC, BCC, and HTML support
- **Draft Management** - Save drafts with reply threading for review before sending
- **Folder Operations** - List, create, and delete mailbox folders including nested hierarchies
- **Email Organization** - Move emails, mark read/unread, flag for follow-up with colors
- **Attachment Download** - Download attachments by filename to disk or as base64
- **Circuit Breaker** - Three-state breaker (closed/open/half-open) fails fast after 5 consecutive errors, auto-recovers after 30s
- **Concurrency Control** - Semaphore-based cap of 10 concurrent tool calls prevents resource exhaustion
- **Secret Redaction** - Custom slog handler scrubs iCloud passwords from all log output (messages, attributes, nested groups)
- **Audit Logging** - Destructive operations (delete, send, reply, draft) logged with full request arguments for post-incident review
- **Structured Logging** - JSON logs with 8-char hex request IDs, tool name, duration, and outcome
- **Input Validation** - Path traversal prevention, size limits, folder/ID sanitization, MCP tool annotations

## Prerequisites

- **Go 1.21+** -- [install](https://go.dev/doc/install) (only needed when building from source)
- **iCloud account** with two-factor authentication enabled
- **App-specific password** -- required for IMAP/SMTP access

## Getting Your iCloud App-Specific Password

1. Go to [appleid.apple.com](https://appleid.apple.com) and sign in
2. Navigate to **Sign-In and Security** > **App-Specific Passwords**
3. Click **Generate an app-specific password**
4. Enter a label (e.g. "MCP Email Server") and click **Create**
5. Copy the generated password (`xxxx-xxxx-xxxx-xxxx`) and store it securely

Notes:
- Your Apple ID must have two-factor authentication enabled
- You can create up to 25 active app-specific passwords
- Changing your main Apple ID password revokes all app-specific passwords
- Never use your main iCloud password for IMAP/SMTP access

## Installation

### From Source

```bash
git clone https://github.com/rgabriel/mcp-icloud-email.git
cd mcp-icloud-email
make build
```

### Using `go install`

```bash
go install github.com/rgabriel/mcp-icloud-email@latest
```

### Docker

```bash
docker build -t mcp-icloud-email .

docker run \
  -e ICLOUD_EMAIL="you@icloud.com" \
  -e ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx" \
  mcp-icloud-email
```

The Docker image uses a multi-stage build with a [distroless](https://github.com/GoogleContainerTools/distroless) base image and runs as a non-root user.

### Prebuilt Binaries

Download the binary for your platform from the [Releases](https://github.com/rgabriel/mcp-icloud-email/releases) page. Binaries are available for:

| Platform | Architecture | Binary |
|----------|-------------|--------|
| Linux | x86_64 | `mcp-icloud-email-linux-amd64` |
| Linux | ARM64 | `mcp-icloud-email-linux-arm64` |
| macOS | Intel | `mcp-icloud-email-macos-amd64` |
| macOS | Apple Silicon | `mcp-icloud-email-macos-arm64` |
| Windows | x86_64 | `mcp-icloud-email-windows-amd64.exe` |

SHA256 checksums are provided alongside each binary.

## Configuration

Create a `.env` file or set environment variables:

```bash
ICLOUD_EMAIL="you@icloud.com"
ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"
```

| Variable | Required | Description |
|----------|----------|-------------|
| `ICLOUD_EMAIL` | Yes | Your iCloud email address (Apple ID) |
| `ICLOUD_PASSWORD` | Yes | App-specific password from appleid.apple.com |
| `LOG_LEVEL` | No | Logging verbosity: `DEBUG`, `INFO` (default), `WARN`, `ERROR` |

## Usage with Claude Desktop

### macOS

`~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "/path/to/mcp-icloud-email",
      "env": {
        "ICLOUD_EMAIL": "you@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

### Windows

`%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "C:\\path\\to\\mcp-icloud-email.exe",
      "env": {
        "ICLOUD_EMAIL": "you@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

### Linux

`~/.config/claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "/path/to/mcp-icloud-email",
      "env": {
        "ICLOUD_EMAIL": "you@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

Restart Claude Desktop after saving.

## Usage with Claude Code CLI

```bash
claude mcp add icloud-email /path/to/mcp-icloud-email \
  -e ICLOUD_EMAIL="you@icloud.com" \
  -e ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"
```

## Available Tools

The server exposes 14 MCP tools. Each tool includes schema constraints and annotations indicating whether it is read-only, destructive, or idempotent.

#### 1. search_emails

Search and list emails with optional filters. Returns email headers for efficiency.

**Parameters:**
- `query` (optional) - Search term for subject/body
- `folder` (optional, default: `INBOX`) - Mailbox folder to search
- `last_days` (optional, default: `30`) - Only show emails from last N days
- `limit` (optional, default: `50`, max: `200`) - Max emails to return
- `offset` (optional, default: `0`) - Skip first N results for pagination
- `unread_only` (optional, default: `false`) - Only return unread emails
- `since` (optional) - Start date in RFC 3339 format
- `before` (optional) - End date in RFC 3339 format

**Example:**
```json
{
  "query": "invoice",
  "folder": "INBOX",
  "last_days": 7,
  "limit": 10,
  "unread_only": true
}
```

#### 2. get_email

Fetch full email content by ID including body, headers, and attachment list.

**Parameters:**
- `email_id` (required) - Email UID from search_emails
- `folder` (optional, default: `INBOX`) - Mailbox folder

**Example:**
```json
{
  "email_id": "12345",
  "folder": "INBOX"
}
```

#### 3. send_email

Compose and send a new email via SMTP.

**Parameters:**
- `to` (required) - Recipient address (string or JSON array)
- `subject` (required) - Subject line
- `body` (required) - Email body content
- `cc` (optional) - CC address(es)
- `bcc` (optional) - BCC address(es)
- `html` (optional, default: `false`) - Whether body is HTML

#### 4. reply_email

Reply to an existing email with automatic In-Reply-To/References headers.

**Parameters:**
- `email_id` (required) - Email UID to reply to
- `body` (required) - Reply body
- `folder` (optional, default: `INBOX`) - Folder containing original
- `reply_all` (optional, default: `false`) - Reply to all recipients
- `html` (optional, default: `false`) - Whether body is HTML

#### 5. draft_email

Save an email as a draft for later review. Supports reply drafts.

**Parameters:**
- `to` (required) - Recipient address(es)
- `subject` (required) - Subject line
- `body` (required) - Email body
- `cc` (optional) - CC address(es)
- `bcc` (optional) - BCC address(es)
- `html` (optional, default: `false`) - Whether body is HTML
- `reply_to_id` (optional) - Original email ID for reply drafts
- `folder` (optional, default: `INBOX`) - Folder of original email

#### 6. delete_email

Delete an email (move to trash or permanently expunge).

**Parameters:**
- `email_id` (required) - Email UID
- `folder` (optional, default: `INBOX`) - Mailbox folder
- `permanent` (optional, default: `false`) - Permanently delete

#### 7. move_email

Move an email between folders.

**Parameters:**
- `email_id` (required) - Email UID
- `from_folder` (optional, default: `INBOX`) - Source folder
- `to_folder` (required) - Destination folder

#### 8. mark_read

Change read/unread status of an email.

**Parameters:**
- `email_id` (required) - Email UID
- `folder` (optional, default: `INBOX`) - Mailbox folder
- `read` (optional, default: `true`) - `true` to mark read, `false` for unread

#### 9. flag_email

Flag an email for follow-up with optional color.

**Parameters:**
- `email_id` (required) - Email UID
- `flag` (required) - `follow-up`, `important`, `deadline`, or `none`
- `folder` (optional, default: `INBOX`) - Mailbox folder
- `color` (optional) - `red`, `orange`, `yellow`, `green`, `blue`, `purple`

#### 10. count_emails

Count emails matching filters without downloading content.

**Parameters:**
- `folder` (optional, default: `INBOX`) - Mailbox folder
- `last_days` (optional) - Only count from last N days
- `unread_only` (optional, default: `false`) - Only count unread

#### 11. list_folders

List all available mailbox folders. Takes no parameters.

#### 12. create_folder

Create a new mailbox folder.

**Parameters:**
- `name` (required) - Folder name
- `parent` (optional) - Parent folder for nesting

#### 13. delete_folder

Delete a mailbox folder. Non-empty folders require `force=true`.

**Parameters:**
- `name` (required) - Folder name
- `force` (optional, default: `false`) - Delete even if folder contains emails

#### 14. get_attachment

Download an email attachment by filename.

**Parameters:**
- `email_id` (required) - Email UID
- `filename` (required) - Attachment filename
- `folder` (optional, default: `INBOX`) - Mailbox folder
- `save_path` (optional) - File path to save to (returns base64 if omitted)

## Working with Large Inboxes

The server uses server-side IMAP SEARCH commands, so filtering happens on the mail server before any data is downloaded.

**Recommended workflow:**

1. Use `count_emails` to check how many emails match your criteria
2. Adjust `last_days`, `since`/`before`, or `unread_only` to narrow results
3. Use `search_emails` with `offset` and `limit` for pagination
4. Use `get_email` only for specific messages you need to read in full

## Development

### Running Locally

```bash
export ICLOUD_EMAIL="you@icloud.com"
export ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"
make run
```

### Building

```bash
make build          # Build binary
make docker         # Build Docker image
```

### Testing

```bash
# Run all tests with race detector
make test

# Run linter
make lint

# Run vulnerability scanner
make vuln

# Run everything
make all
```

### Testing with MCP Inspector

```bash
npx @modelcontextprotocol/inspector mcp-icloud-email
```

## Troubleshooting

### Authentication Failed

**Problem:** IMAP connection fails with authentication error.

**Solutions:**
- Verify you are using an app-specific password, not your main iCloud password
- Check that two-factor authentication is enabled on your Apple ID
- Regenerate a new app-specific password at appleid.apple.com
- Confirm your email address matches your Apple ID

### Folder Not Found

**Problem:** Tool returns "folder not found" error.

**Solutions:**
- Run `list_folders` to see the exact folder names your account has
- iCloud uses names like "Deleted Messages" rather than "Trash"
- Folder names are case-sensitive

### Timeouts or Slow Responses

**Problem:** Tool calls time out or return slowly.

**Solutions:**
- Check your internet connection
- Reduce the `limit` parameter for large result sets
- Use `count_emails` first to gauge result size
- Use narrower date ranges with `since`/`before` or `last_days`
- If the circuit breaker opens after repeated failures, wait 30 seconds for it to probe again
- Try accessing iCloud in your browser to verify service status

### Email Not Found

**Problem:** `get_email` returns "not found" for a known ID.

**Solutions:**
- Email IDs (UIDs) are unique per folder -- make sure you are looking in the correct folder
- The email may have been moved or deleted since the ID was retrieved
- Use `search_emails` to find the current email ID

## Architecture

```
mcp-icloud-email/
  main.go              Server setup, tool registration, middleware chain
  redact.go            Custom slog.Handler for secret redaction
  circuitbreaker.go    Three-state circuit breaker (closed/open/half-open)
  config/config.go     Environment variable loading and validation
  imap/client.go       IMAP client (imap.mail.me.com:993, TLS)
  smtp/client.go       SMTP client (smtp.mail.me.com:587, STARTTLS)
  tools/
    interfaces.go      EmailReader, EmailWriter, EmailService, EmailSender
    helpers.go         Address parsing, shared utilities
    validate.go        Input validation (paths, folders, IDs, sizes)
    <tool>.go          One file per tool handler (14 files)
```

Middleware is applied in this order (outermost first):
1. **Concurrency cap** (10 concurrent tool calls)
2. **Timeout + logging + audit + circuit breaker** (60s deadline, correlation IDs, audit trail for destructive ops)
3. **Panic recovery** (built into the SDK)

**Thread safety:** The IMAP client uses a `sync.Mutex` to serialize access. Internal methods assume the caller holds the lock, preventing deadlocks from nested calls.

## Dependencies

| Package | Purpose |
|---------|---------|
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP SDK -- tool registration, stdio transport |
| [go-imap/v2](https://github.com/emersion/go-imap) | IMAP protocol client |
| [go-message](https://github.com/emersion/go-message) | MIME parsing and email formatting |
| [godotenv](https://github.com/joho/godotenv) | `.env` file loading |
| [uuid](https://github.com/google/uuid) | Message-ID generation for emails |

## Security

- **App-specific passwords only** -- never accepts or stores your main iCloud password
- **TLS everywhere** -- IMAP on port 993 (implicit TLS), SMTP on port 587 (STARTTLS)
- **Secret redaction** -- iCloud passwords are replaced with `[REDACTED]` in all log output (messages, attributes, nested groups)
- **Audit logging** -- destructive operations (delete, send, reply, draft) logged with full request arguments
- **Circuit breaker** -- fails fast after 5 consecutive upstream errors, auto-recovers after 30s
- **Concurrency cap** -- limits to 10 concurrent tool calls to prevent resource exhaustion
- **Input validation** -- path traversal prevention, null byte rejection, IMAP wildcard filtering, control character rejection, numeric UID validation
- **Size limits** -- 10 MB body, 998-character subject (per RFC 2822)
- **Distroless Docker image** -- minimal attack surface, runs as non-root
- **No third-party data sharing** -- the server runs locally and communicates only with iCloud servers
- **Revocable access** -- app-specific passwords can be revoked at any time from appleid.apple.com

Never commit your `.env` file to version control. The `.gitignore` already excludes it.

## License

MIT License -- see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome. Please open an issue to discuss larger changes before submitting a pull request.

## Support

For issues, questions, or feature requests, please open an issue on [GitHub](https://github.com/rgabriel/mcp-icloud-email/issues).

## Acknowledgments

- Built with the official [mcp-go SDK](https://mcp-go.dev)
- Integrates with [Apple iCloud Mail](https://support.apple.com/mail) via IMAP/SMTP
- Follows the [Model Context Protocol](https://modelcontextprotocol.io) specification
