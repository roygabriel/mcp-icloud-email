# iCloud Email MCP Server

A [Model Context Protocol](https://modelcontextprotocol.io) server that gives AI assistants full access to Apple iCloud Mail through IMAP and SMTP. Search, read, send, reply, organize, and manage your iCloud mailbox -- all from Claude or any MCP-compatible client.

Built with Go and the [mcp-go SDK](https://mcp-go.dev). Ships as a single static binary for Linux, macOS, and Windows.

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage with Claude Desktop](#usage-with-claude-desktop)
- [Available Tools](#available-tools)
- [Working with Large Inboxes](#working-with-large-inboxes)
- [Development](#development)
- [Architecture](#architecture)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Features

**Email Operations**
- Search and list emails with filters for date range, read status, and text queries
- Retrieve full email content including body, headers, and attachment metadata
- Send new emails with CC, BCC, and HTML support
- Reply to emails with reply-all support
- Save drafts for review before sending
- Download attachments by filename (to disk or as base64)

**Mailbox Management**
- List, create, and delete mailbox folders (including nested folders)
- Move emails between folders
- Mark emails as read or unread
- Flag emails for follow-up with customizable colors
- Delete emails (move to trash or permanent)
- Count emails matching filters without fetching content

**Operational**
- Thread-safe IMAP access with mutex protection
- Structured JSON logging with UUID request correlation
- 60-second timeout middleware on every tool call
- Input validation: path traversal prevention, size limits, folder/ID sanitization
- MCP tool annotations (read-only, destructive, idempotent) for client-side safety
- CI pipeline with tests, linting, and vulnerability scanning

---

## Quick Start

```bash
# Install
go install github.com/rgabriel/mcp-icloud-email@latest

# Set credentials (app-specific password, not your main iCloud password)
export ICLOUD_EMAIL="you@icloud.com"
export ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"

# Run
mcp-icloud-email
```

Or download a prebuilt binary from the [Releases](https://github.com/rgabriel/mcp-icloud-email/releases) page.

---

## Prerequisites

- **Go 1.21+** -- [install](https://go.dev/doc/install) (only needed when building from source)
- **iCloud account** with two-factor authentication enabled
- **App-specific password** -- required for IMAP/SMTP access

### Generating an App-Specific Password

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

---

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

---

## Configuration

The server requires two environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `ICLOUD_EMAIL` | Yes | Your iCloud email address (Apple ID) |
| `ICLOUD_PASSWORD` | Yes | App-specific password from appleid.apple.com |
| `LOG_LEVEL` | No | Logging verbosity: `DEBUG`, `INFO` (default), `WARN`, `ERROR` |

You can set these as environment variables or place them in a `.env` file:

```bash
cp .env.example .env
# Edit .env with your credentials
```

---

## Usage with Claude Desktop

Add the server to your Claude Desktop configuration file.

**macOS** -- `~/Library/Application Support/Claude/claude_desktop_config.json`

**Linux** -- `~/.config/claude/claude_desktop_config.json`

**Windows** -- `%APPDATA%\Claude\claude_desktop_config.json`

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

---

## Available Tools

The server exposes 14 MCP tools. Each tool includes schema constraints and annotations indicating whether it is read-only, destructive, or idempotent.

### search_emails

Search and list emails with optional filters. Returns email headers (not full bodies) for efficiency.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `query` | string | | Search term for subject/body |
| `folder` | string | `INBOX` | Mailbox folder to search |
| `last_days` | integer | `30` | Only show emails from last N days |
| `limit` | integer | `50` | Max emails to return (max 200) |
| `offset` | integer | `0` | Skip first N results (for pagination) |
| `unread_only` | boolean | `false` | Only return unread emails |
| `since` | string | | Start date (ISO 8601) |
| `before` | string | | End date (ISO 8601) |

Response includes `count` (returned), `total` (matching before offset/limit), and an array of email summaries.

### get_email

Retrieve full email content including body text, HTML, headers, and attachment list.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `folder` | string | `INBOX` | Mailbox folder |

### send_email

Compose and send a new email.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `to` | string/array | *(required)* | Recipient address(es) |
| `subject` | string | *(required)* | Subject line |
| `body` | string | *(required)* | Email body |
| `cc` | string/array | | CC address(es) |
| `bcc` | string/array | | BCC address(es) |
| `html` | boolean | `false` | Whether body is HTML |

### reply_email

Reply to an existing email. Automatically sets In-Reply-To and References headers.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID to reply to |
| `body` | string | *(required)* | Reply body |
| `folder` | string | `INBOX` | Folder containing original email |
| `reply_all` | boolean | `false` | Reply to all recipients |
| `html` | boolean | `false` | Whether body is HTML |

### draft_email

Save an email as a draft. Supports reply drafts with automatic header threading.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `to` | string/array | *(required)* | Recipient address(es) |
| `subject` | string | *(required)* | Subject line |
| `body` | string | *(required)* | Email body |
| `cc` | string/array | | CC address(es) |
| `bcc` | string/array | | BCC address(es) |
| `html` | boolean | `false` | Whether body is HTML |
| `reply_to_id` | string | | Original email ID for reply drafts |
| `folder` | string | `INBOX` | Folder of original email (for replies) |

### delete_email

Delete an email by moving it to trash, or permanently delete it.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `folder` | string | `INBOX` | Mailbox folder |
| `permanent` | boolean | `false` | Permanently delete instead of trashing |

### move_email

Move an email from one folder to another.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `from_folder` | string | `INBOX` | Source folder |
| `to_folder` | string | *(required)* | Destination folder |

### mark_read

Change the read/unread status of an email.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `folder` | string | `INBOX` | Mailbox folder |
| `read` | boolean | `true` | `true` to mark read, `false` for unread |

### flag_email

Flag an email for follow-up with optional color.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `flag` | string | *(required)* | `follow-up`, `important`, `deadline`, or `none` |
| `folder` | string | `INBOX` | Mailbox folder |
| `color` | string | | `red`, `orange`, `yellow`, `green`, `blue`, `purple` |

Set `flag` to `none` to remove all flags.

### count_emails

Count emails matching filters without downloading message content.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `folder` | string | `INBOX` | Mailbox folder |
| `last_days` | integer | | Only count from last N days |
| `unread_only` | boolean | `false` | Only count unread |

### list_folders

List all available mailbox folders. Takes no parameters.

### create_folder

Create a new mailbox folder.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | *(required)* | Folder name |
| `parent` | string | | Parent folder for nesting (e.g. `Work/Projects`) |

### delete_folder

Delete a mailbox folder. Non-empty folders require explicit confirmation.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `name` | string | *(required)* | Folder name |
| `force` | boolean | `false` | Delete even if folder contains emails |

System folders (INBOX, Sent, Trash) cannot be deleted.

### get_attachment

Download an email attachment by filename.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `email_id` | string | *(required)* | Email UID |
| `filename` | string | *(required)* | Attachment filename |
| `folder` | string | `INBOX` | Mailbox folder |
| `save_path` | string | | File path to save to (returns base64 if omitted) |

---

## Working with Large Inboxes

The server uses server-side IMAP SEARCH commands, so filtering happens on the mail server before any data is downloaded. Default settings are tuned for large mailboxes:

- `search_emails` defaults to the last 30 days and a limit of 50
- `count_emails` returns counts without fetching message content
- `get_email` loads full body content on demand for individual messages

**Recommended workflow:**

1. Use `count_emails` to check how many emails match your criteria
2. Adjust `last_days`, `since`/`before`, or `unread_only` to narrow results
3. Use `search_emails` with `offset` and `limit` for pagination
4. Use `get_email` only for specific messages you need to read in full

---

## Development

### Building

```bash
make build          # Build binary
make test           # Run tests with race detector
make lint           # Run golangci-lint
make vet            # Run go vet
make vuln           # Run govulncheck
make all            # vet + lint + test + build
make docker         # Build Docker image
make tools          # Install dev tools (golangci-lint, govulncheck)
```

### Running Locally

```bash
export ICLOUD_EMAIL="you@icloud.com"
export ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"
make run
```

### Testing

The project includes 78+ table-driven tests covering all tool handlers, input validation, and error paths. Tests use mock implementations of the `EmailService` and `EmailSender` interfaces -- no live IMAP/SMTP connection required.

```bash
make test
```

### Testing with MCP Inspector

Use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to interactively test the server:

```bash
npx @modelcontextprotocol/inspector mcp-icloud-email
```

### CI Pipeline

Every push to `main` or `dev` and every pull request runs:

- `go vet` and `go test -race` -- correctness and data race detection
- `golangci-lint` -- static analysis (errcheck, govet, staticcheck, gosec, gocritic, and more)
- `govulncheck` -- known vulnerability scanning

Tagged releases (`v*.*.*`) trigger automated cross-platform builds with SHA256 checksums.

---

## Architecture

```
mcp-icloud-email/
  main.go              Server setup, tool registration, middleware chain
  config/config.go     Environment variable loading and validation
  imap/client.go       IMAP client (imap.mail.me.com:993, TLS)
  smtp/client.go       SMTP client (smtp.mail.me.com:587, STARTTLS)
  tools/
    interfaces.go      EmailReader, EmailWriter, EmailService, EmailSender
    helpers.go         Address parsing, shared utilities
    validate.go        Input validation (paths, folders, IDs, sizes)
    handlers_test.go   78+ table-driven tests with mocks
    <tool>.go          One file per tool handler (14 files)
```

**Middleware chain:** Each tool call passes through `logging -> timeout -> handler`. The logging middleware assigns a UUID request ID and records tool name, duration, and outcome. The timeout middleware enforces a 60-second deadline.

**Thread safety:** The IMAP client uses a `sync.Mutex` to serialize access. Internal methods (lowercase) assume the caller holds the lock, preventing deadlocks from nested calls like `DeleteEmail -> moveEmail`.

### Dependencies

| Package | Purpose |
|---------|---------|
| [mcp-go](https://github.com/mark3labs/mcp-go) | MCP SDK -- tool registration, stdio transport |
| [go-imap/v2](https://github.com/emersion/go-imap) | IMAP protocol client |
| [go-message](https://github.com/emersion/go-message) | MIME parsing and email formatting |
| [godotenv](https://github.com/joho/godotenv) | `.env` file loading |
| [uuid](https://github.com/google/uuid) | Message-ID and request ID generation |

---

## Security

- **App-specific passwords only** -- never accepts or stores your main iCloud password
- **TLS everywhere** -- IMAP on port 993 (implicit TLS), SMTP on port 587 (STARTTLS)
- **Input validation** -- path traversal prevention, null byte rejection, IMAP wildcard filtering, control character rejection, numeric UID validation
- **Size limits** -- 10 MB body, 998-character subject (per RFC 2822)
- **Distroless Docker image** -- minimal attack surface, runs as non-root
- **No third-party data sharing** -- the server runs locally and communicates only with iCloud servers
- **Revocable access** -- app-specific passwords can be revoked at any time from appleid.apple.com

Never commit your `.env` file to version control. The `.gitignore` already excludes it.

---

## Troubleshooting

### Authentication Failed

- Verify you are using an app-specific password, not your main iCloud password
- Check that two-factor authentication is enabled on your Apple ID
- Regenerate a new app-specific password at appleid.apple.com
- Confirm your email address matches your Apple ID

### Folder Not Found

- Run `list_folders` to see the exact folder names your account has
- iCloud uses names like "Deleted Messages" rather than "Trash"
- Folder names are case-sensitive

### Invalid Date Format

- Use ISO 8601: `2024-01-15T14:30:00Z`
- Include timezone offset if not UTC: `2024-01-15T14:30:00-05:00`

### Timeouts or Slow Responses

- Check your internet connection
- Reduce the `limit` parameter for large result sets
- Use `count_emails` first to gauge result size before searching
- Use narrower date ranges with `since`/`before` or `last_days`

### Email Not Found

- Email IDs (UIDs) are unique per folder -- make sure you are looking in the correct folder
- The email may have been moved or deleted since the ID was retrieved
- Use `search_emails` to find the current email ID

---

## Contributing

Contributions are welcome. Please open an issue to discuss larger changes before submitting a pull request.

---

## License

MIT License -- see [LICENSE](LICENSE) for details.
