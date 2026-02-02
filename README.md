# iCloud Email MCP Server

A Model Context Protocol (MCP) server that connects to Apple iCloud Mail using IMAP and SMTP protocols. This server enables AI assistants like Claude to interact with your iCloud email - search emails, read messages, send emails, reply to messages, and manage your mailbox.

Built with Go using the official [mcp-go SDK](https://mcp-go.dev) and works on all operating systems (Linux, Windows, macOS).

## Features

- **Search Emails** - Search and list emails with filters for date range, unread status, and text queries
- **Get Email** - Retrieve full email content including body, headers, and attachments list
- **Send Email** - Compose and send new emails with CC, BCC, and HTML support
- **Reply to Email** - Reply to existing emails with reply-all support
- **Delete Email** - Move emails to trash or permanently delete them
- **Move Email** - Move emails between mailbox folders
- **List Folders** - Discover all available mailbox folders
- **Mark Read/Unread** - Change read status of emails
- **Count Emails** - Count emails matching filters without fetching full content
- **Cross-Platform** - Works on Linux, Windows, and macOS
- **Secure** - Uses app-specific passwords (never your main iCloud password)
- **Smart Defaults** - Handles large inboxes efficiently with server-side filtering

## Prerequisites

- **Go 1.21 or higher** - [Install Go](https://go.dev/doc/install)
- **iCloud Account** with two-factor authentication (2FA) enabled
- **App-Specific Password** - Required for IMAP/SMTP access (see setup below)

## App-Specific Password Setup

iCloud requires an app-specific password for third-party applications to access your email data. Follow these steps:

1. Go to [Apple ID Account Management](https://appleid.apple.com)
2. Sign in with your Apple ID
3. Navigate to **Sign-In and Security** section
4. Click on **App-Specific Passwords**
5. Click **Generate an app-specific password**
6. Enter a label like "MCP Email Server"
7. Click **Create**
8. Copy the generated password (format: `xxxx-xxxx-xxxx-xxxx`)
9. Save this password securely - you won't be able to see it again

**Important Notes:**
- Your Apple ID must have two-factor authentication enabled
- You can create up to 25 active app-specific passwords
- If you change your main Apple ID password, all app-specific passwords are revoked
- Never use your main iCloud password for IMAP/SMTP access

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/rgabriel/mcp-icloud-email.git
cd mcp-icloud-email

# Build the server
go build -o mcp-icloud-email

# Optional: Install to your PATH
go install
```

### Using go install

```bash
go install github.com/rgabriel/mcp-icloud-email@latest
```

## Configuration

Create a `.env` file in the same directory as the server executable (for local testing):

```bash
cp .env.example .env
```

Edit `.env` and add your credentials:

```bash
ICLOUD_EMAIL=your-email@icloud.com
ICLOUD_PASSWORD=xxxx-xxxx-xxxx-xxxx
```

**Environment Variables:**

- `ICLOUD_EMAIL` (required) - Your iCloud email address (Apple ID)
- `ICLOUD_PASSWORD` (required) - App-specific password from appleid.apple.com

## Usage with Claude Desktop

Add this server to your Claude Desktop configuration file:

### macOS

Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "/path/to/mcp-icloud-email",
      "env": {
        "ICLOUD_EMAIL": "your-email@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

### Windows

Edit `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "C:\\path\\to\\mcp-icloud-email.exe",
      "env": {
        "ICLOUD_EMAIL": "your-email@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

### Linux

Edit `~/.config/claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "icloud-email": {
      "command": "/path/to/mcp-icloud-email",
      "env": {
        "ICLOUD_EMAIL": "your-email@icloud.com",
        "ICLOUD_PASSWORD": "xxxx-xxxx-xxxx-xxxx"
      }
    }
  }
}
```

After adding the configuration, restart Claude Desktop.

## Available Tools

### 1. search_emails

Search and list emails with optional filters.

**Parameters:**
- `query` (optional) - Search term to find in subject/body
- `folder` (optional) - Mailbox folder to search in (default: INBOX)
- `last_days` (optional) - Only show emails from last N days (default: 30)
- `limit` (optional) - Maximum number of emails to return (default: 50, max: 200)
- `unread_only` (optional) - Only return unread emails (default: false)
- `since` (optional) - Start date filter in ISO 8601 format
- `before` (optional) - End date filter in ISO 8601 format

**Example:**
```json
{
  "folder": "INBOX",
  "last_days": 7,
  "unread_only": true,
  "limit": 20
}
```

**Example Response:**
```json
{
  "count": 3,
  "folder": "INBOX",
  "emails": [
    {
      "id": "12345",
      "from": "John Doe <john@example.com>",
      "to": ["you@icloud.com"],
      "subject": "Meeting Tomorrow",
      "date": "2024-02-01T14:30:00Z",
      "snippet": "Hi, I wanted to confirm our meeting...",
      "unread": true,
      "attachments": []
    }
  ]
}
```

**Note:** The default `last_days` of 30 helps handle large inboxes efficiently by using server-side IMAP filtering.

### 2. get_email

Get full email content including body and attachments.

**Parameters:**
- `email_id` (required) - Email ID (UID) to retrieve
- `folder` (optional) - Mailbox folder containing the email (default: INBOX)

**Example:**
```json
{
  "email_id": "12345",
  "folder": "INBOX"
}
```

**Example Response:**
```json
{
  "id": "12345",
  "from": "John Doe <john@example.com>",
  "to": ["you@icloud.com"],
  "cc": [],
  "subject": "Meeting Tomorrow",
  "date": "2024-02-01T14:30:00Z",
  "bodyPlain": "Hi, I wanted to confirm our meeting for tomorrow at 2pm.",
  "bodyHTML": "<html><body>Hi, I wanted to confirm...</body></html>",
  "unread": false,
  "attachments": [
    {
      "filename": "agenda.pdf",
      "size": 52480
    }
  ],
  "messageId": "<abc123@example.com>"
}
```

### 3. send_email

Send a new email.

**Parameters:**
- `to` (required) - Recipient email address or array of addresses
- `subject` (required) - Email subject line
- `body` (required) - Email body content
- `cc` (optional) - CC email address or array of addresses
- `bcc` (optional) - BCC email address or array of addresses
- `html` (optional) - Whether body is HTML format (default: false)

**Example:**
```json
{
  "to": "recipient@example.com",
  "subject": "Project Update",
  "body": "Here's the latest status on the project...",
  "cc": ["manager@example.com"],
  "html": false
}
```

**Example Response:**
```json
{
  "success": true,
  "message": "Email sent successfully to [recipient@example.com]",
  "subject": "Project Update"
}
```

### 4. reply_email

Reply to an existing email.

**Parameters:**
- `email_id` (required) - Email ID (UID) being replied to
- `body` (required) - Reply message body
- `folder` (optional) - Mailbox folder containing the original email (default: INBOX)
- `reply_all` (optional) - Reply to all recipients (default: false)
- `html` (optional) - Whether body is HTML format (default: false)

**Example:**
```json
{
  "email_id": "12345",
  "body": "Thanks for the update! I'll be there.",
  "reply_all": false
}
```

**Example Response:**
```json
{
  "success": true,
  "message": "Reply sent successfully",
  "original_subject": "Meeting Tomorrow"
}
```

### 5. delete_email

Delete an email (move to trash or permanently delete).

**Parameters:**
- `email_id` (required) - Email ID (UID) to delete
- `folder` (optional) - Mailbox folder containing the email (default: INBOX)
- `permanent` (optional) - Permanently delete instead of moving to trash (default: false)

**Example:**
```json
{
  "email_id": "12345",
  "folder": "INBOX",
  "permanent": false
}
```

**Example Response:**
```json
{
  "success": true,
  "email_id": "12345",
  "message": "Email moved to trash successfully"
}
```

### 6. move_email

Move an email from one folder to another.

**Parameters:**
- `email_id` (required) - Email ID (UID) to move
- `from_folder` (optional) - Source mailbox folder (default: INBOX)
- `to_folder` (required) - Destination mailbox folder

**Example:**
```json
{
  "email_id": "12345",
  "from_folder": "INBOX",
  "to_folder": "Archive"
}
```

**Example Response:**
```json
{
  "success": true,
  "email_id": "12345",
  "from_folder": "INBOX",
  "to_folder": "Archive",
  "message": "Email moved from 'INBOX' to 'Archive' successfully"
}
```

### 7. list_folders

List all available mailbox folders.

**Parameters:** None

**Example Response:**
```json
{
  "count": 5,
  "folders": [
    "INBOX",
    "Sent Messages",
    "Drafts",
    "Deleted Messages",
    "Archive"
  ]
}
```

### 8. mark_read

Mark an email as read or unread.

**Parameters:**
- `email_id` (required) - Email ID (UID) to mark
- `folder` (optional) - Mailbox folder containing the email (default: INBOX)
- `read` (optional) - Mark as read (true) or unread (false) (default: true)

**Example:**
```json
{
  "email_id": "12345",
  "folder": "INBOX",
  "read": true
}
```

**Example Response:**
```json
{
  "success": true,
  "email_id": "12345",
  "message": "Email marked as read successfully"
}
```

### 9. count_emails

Count emails matching filters without fetching full content.

**Parameters:**
- `folder` (optional) - Mailbox folder to count in (default: INBOX)
- `last_days` (optional) - Only count emails from last N days
- `unread_only` (optional) - Only count unread emails (default: false)

**Example:**
```json
{
  "folder": "INBOX",
  "last_days": 7,
  "unread_only": true
}
```

**Example Response:**
```json
{
  "count": 15,
  "folder": "INBOX",
  "last_days": 7,
  "unread_only": true
}
```

## Date/Time Format

All date/time parameters use **ISO 8601 format** (RFC3339 in Go):

- Format: `YYYY-MM-DDTHH:MM:SSZ`
- Examples:
  - `2024-01-15T14:30:00Z` (UTC)
  - `2024-01-15T14:30:00-05:00` (with timezone offset)
  - `2024-01-15T14:30:00+01:00` (with timezone offset)

The server handles timezone conversions automatically.

## Handling Large Inboxes

This server is optimized for large inboxes with thousands of emails:

- **Server-side filtering**: Uses IMAP SEARCH commands to filter on the server before downloading
- **Default limits**: `search_emails` defaults to last 30 days and max 50 emails per query
- **Smart fetching**: Only fetches email headers for searches; full body content loaded on demand with `get_email`
- **Efficient counting**: `count_emails` tool provides quick counts without fetching message content

**Recommended workflow for large inboxes:**
1. Use `count_emails` first to see how many emails match your criteria
2. Adjust `last_days` or date filters to narrow results if needed
3. Use `search_emails` with appropriate limits
4. Use `get_email` to retrieve full content only for specific messages

## Development

### Running Locally

```bash
# Set environment variables
export ICLOUD_EMAIL="your-email@icloud.com"
export ICLOUD_PASSWORD="xxxx-xxxx-xxxx-xxxx"

# Run the server
go run main.go
```

### Building

```bash
# Build for your current platform
go build -o mcp-icloud-email

# Build for specific platforms
GOOS=linux GOARCH=amd64 go build -o mcp-icloud-email-linux
GOOS=darwin GOARCH=arm64 go build -o mcp-icloud-email-macos
GOOS=windows GOARCH=amd64 go build -o mcp-icloud-email-windows.exe
```

### Testing with MCP Inspector

Use the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) to test the server:

```bash
npx @modelcontextprotocol/inspector mcp-icloud-email
```

## Troubleshooting

### Authentication Failed

**Problem:** "Failed to login" or "Failed to connect to iCloud IMAP"

**Solutions:**
- Verify you're using an **app-specific password**, not your main iCloud password
- Check that your Apple ID has two-factor authentication enabled
- Regenerate a new app-specific password at appleid.apple.com
- Ensure your email address is correct (it should be your Apple ID)
- Check your internet connection and firewall settings

### Folder Not Found

**Problem:** "failed to select folder" error

**Solutions:**
- Run the `list_folders` tool to see available folder names
- iCloud uses specific folder names like "Deleted Messages" (not "Trash")
- Folder names are case-sensitive
- Some folders may have special characters or spaces in their names

### Invalid Date Format

**Problem:** "invalid since format" or "invalid before format"

**Solutions:**
- Use ISO 8601 format: `YYYY-MM-DDTHH:MM:SSZ`
- Example: `2024-01-15T14:30:00Z`
- Include timezone offset if not UTC: `2024-01-15T14:30:00-05:00`

### Network Timeouts

**Problem:** Connection timeouts or slow responses

**Solutions:**
- Check your internet connection
- iCloud IMAP/SMTP servers may be temporarily unavailable
- The server has a 30-second timeout - wait and retry
- Check if you can access icloud.com in your browser
- Reduce `limit` parameter for searches if timeouts persist

### Email Not Found

**Problem:** "email not found" error

**Solutions:**
- Verify the email ID is correct (IDs are unique per folder)
- Make sure you're searching in the correct folder
- The email may have been moved or deleted
- Use `search_emails` to find the correct email ID

### Large Inbox Performance

**Problem:** Slow searches or timeouts with large inbox

**Solutions:**
- Use the default `last_days` filter (30 days) to limit search scope
- Reduce the `limit` parameter (default 50, max 200)
- Use `count_emails` first to check how many emails match before fetching
- Use more specific date ranges with `since` and `before` parameters
- Server-side IMAP filtering helps, but very large result sets still take time

## Architecture

The server consists of:

- **IMAP Client** (`imap/client.go`) - Handles iCloud IMAP protocol communication for reading emails
- **SMTP Client** (`smtp/client.go`) - Handles iCloud SMTP protocol for sending emails
- **Configuration** (`config/config.go`) - Loads and validates environment variables
- **Tool Handlers** (`tools/*.go`) - Implements MCP tool logic for each operation
- **Main Server** (`main.go`) - MCP server initialization and tool registration

## Dependencies

- [mcp-go](https://github.com/mark3labs/mcp-go) v0.43.2 - Official MCP Go SDK
- [go-imap/v2](https://github.com/emersion/go-imap) - IMAP client library
- [go-message](https://github.com/emersion/go-message) - Email parsing and formatting
- [godotenv](https://github.com/joho/godotenv) v1.5.1 - Environment variable loader

## Security Considerations

- **Never commit your `.env` file to version control**
- Use app-specific passwords, never your main iCloud password
- App-specific passwords can be revoked at any time from appleid.apple.com
- The server runs locally and doesn't send data to third parties
- All communication with iCloud uses TLS encryption (IMAP port 993, SMTP with STARTTLS)
- Email content may contain sensitive information - handle with care
- Consider using secret management tools instead of environment variables in production
- Environment variables can be logged or exposed - be cautious in shared environments

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues, questions, or feature requests, please open an issue on [GitHub](https://github.com/rgabriel/mcp-icloud-email/issues).

## Acknowledgments

- Built with the official [mcp-go SDK](https://mcp-go.dev)
- IMAP implementation using [go-imap/v2](https://github.com/emersion/go-imap)
- Email parsing using [go-message](https://github.com/emersion/go-message)
- Follows the [Model Context Protocol](https://modelcontextprotocol.io) specification
