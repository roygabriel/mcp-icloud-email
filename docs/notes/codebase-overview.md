# Codebase Overview

## Architecture
- **Single-binary MCP server** communicating over stdio (JSON-RPC)
- **4 packages**: main, config, imap, smtp (tools are in a separate package under tools/)
- **14 MCP tools** registered in main.go
- **Persistent IMAP connection** for the server lifetime; SMTP connects per-send

## Key Files
| File | Purpose | Lines |
|------|---------|-------|
| `main.go` | Entry point, tool registration, server startup | 319 |
| `config/config.go` | Env var loading (.env + ICLOUD_EMAIL/ICLOUD_PASSWORD) | 37 |
| `imap/client.go` | IMAP client: search, get, move, delete, flag, draft, attach, folders | ~919 |
| `smtp/client.go` | SMTP client: send, reply, HTML stripping | ~257 |
| `tools/*.go` | 13 handler files (folder.go has create+delete) | ~700 total |
| `.github/workflows/release.yml` | CI: cross-compile 5 platforms, GitHub Release | 184 |

## Dependencies (go.mod)
- `go 1.25.1`
- `github.com/emersion/go-imap v1.2.1` - IMAP protocol
- `github.com/emersion/go-message v0.18.2` - MIME message parsing
- `github.com/joho/godotenv v1.5.1` - .env file loader
- `github.com/mark3labs/mcp-go v0.43.2` - MCP SDK
- `github.com/google/uuid v1.6.0` - (indirect, available but unused)

## Data Flow
1. Claude Desktop/CLI spawns binary, communicates via stdin/stdout
2. `main.go` loads config, creates IMAP+SMTP clients, registers 14 tools
3. `server.ServeStdio(s)` handles MCP JSON-RPC requests
4. Each tool handler extracts args, calls imap/smtp methods, returns JSON
5. On exit, `defer imapClient.Close()` logs out from IMAP

## What's Missing
- Zero test files (`*_test.go`)
- No Makefile, Dockerfile, .golangci.yml
- No structured logging (uses fmt/log)
- No signal handling
- No service interfaces (concrete types everywhere)
- No tool annotations (read-only, destructive, idempotent hints)
