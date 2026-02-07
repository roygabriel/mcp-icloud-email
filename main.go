package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rgabriel/mcp-icloud-email/config"
	"github.com/rgabriel/mcp-icloud-email/imap"
	"github.com/rgabriel/mcp-icloud-email/smtp"
	"github.com/rgabriel/mcp-icloud-email/tools"
)

// version is set at build time via ldflags
var version = "dev"

func main() {
	// Initialize structured logging
	logLevel := new(slog.LevelVar)
	logLevel.Set(slog.LevelInfo)
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		switch strings.ToUpper(lvl) {
		case "DEBUG":
			logLevel.Set(slog.LevelDebug)
		case "WARN":
			logLevel.Set(slog.LevelWarn)
		case "ERROR":
			logLevel.Set(slog.LevelError)
		}
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	// Create IMAP client
	imapClient, err := imap.NewClient(cfg.ICloudEmail, cfg.ICloudPassword)
	if err != nil {
		slog.Error("failed to create IMAP client", "error", err)
		os.Exit(1)
	}
	defer imapClient.Close()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Test IMAP connection by listing folders
	_, err = imapClient.ListFolders(ctx)
	if err != nil {
		slog.Error("failed to connect to iCloud IMAP (check credentials)", "error", err)
		os.Exit(1)
	}

	// Create SMTP client
	smtpClient := smtp.NewClient(cfg.ICloudEmail, cfg.ICloudPassword)

	// Create MCP server with middleware (applied in reverse: logging wraps timeout wraps handler)
	s := server.NewMCPServer(
		"iCloud Email Server",
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
		server.WithToolHandlerMiddleware(timeoutMiddleware(60*time.Second)),
		server.WithToolHandlerMiddleware(loggingMiddleware()),
	)

	// Register search_emails tool
	searchEmailsTool := mcp.NewTool("search_emails",
		mcp.WithDescription("Search and list emails with optional filters. Use list_folders first to discover valid folder names. Returns each email's id (use with get_email), from, to, subject, date, and unread status."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("query",
			mcp.Description("Search term to find in subject and body text"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder to search in. Use list_folders to discover valid names."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithNumber("last_days",
			mcp.Description("Only return emails from the last N days. Ignored if 'since' is provided."),
			mcp.DefaultNumber(30),
			mcp.Min(1),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of emails to return. Most recent emails are returned first."),
			mcp.DefaultNumber(50),
			mcp.Min(1),
			mcp.Max(200),
		),
		mcp.WithNumber("offset",
			mcp.Description("Number of most-recent matching emails to skip (for pagination). Use with limit to page through results."),
			mcp.DefaultNumber(0),
			mcp.Min(0),
		),
		mcp.WithBoolean("unread_only",
			mcp.Description("Only return unread (unseen) emails."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("since",
			mcp.Description("Start date filter in RFC 3339 format (e.g., '2024-01-15T14:30:00Z'). Overrides last_days."),
		),
		mcp.WithString("before",
			mcp.Description("End date filter in RFC 3339 format (e.g., '2024-01-15T14:30:00Z')."),
		),
	)
	s.AddTool(searchEmailsTool, tools.SearchEmailsHandler(imapClient))

	// Register get_email tool
	getEmailTool := mcp.NewTool("get_email",
		mcp.WithDescription("Fetch full email content by ID. Use search_emails first to find email IDs. Returns from, to, cc, subject, date, plain text body, HTML body, unread status, attachment metadata (filename, size), messageId, and references."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID from search_emails results."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email. Use list_folders to discover valid names."),
			mcp.DefaultString("INBOX"),
		),
	)
	s.AddTool(getEmailTool, tools.GetEmailHandler(imapClient))

	// Register send_email tool
	sendEmailTool := mcp.NewTool("send_email",
		mcp.WithDescription("Compose and send a new email via SMTP. Returns success status and subject. Calling twice will send duplicate emails."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("to",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Recipient email address (string) or JSON array of addresses."),
		),
		mcp.WithString("subject",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email subject line."),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email body content. Plain text by default; set html=true for HTML."),
		),
		mcp.WithString("cc",
			mcp.Description("CC email address (string) or JSON array of addresses."),
		),
		mcp.WithString("bcc",
			mcp.Description("BCC email address (string) or JSON array of addresses."),
		),
		mcp.WithBoolean("html",
			mcp.Description("Set true if body contains HTML. A plain text version is auto-generated."),
			mcp.DefaultBool(false),
		),
	)
	s.AddTool(sendEmailTool, tools.SendEmailHandler(smtpClient, cfg.ICloudEmail))

	// Register reply_email tool
	replyEmailTool := mcp.NewTool("reply_email",
		mcp.WithDescription("Reply to an existing email. Use get_email first to read the original. Automatically sets In-Reply-To/References headers and Re: subject prefix. Calling twice sends duplicate replies."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID of the message being replied to (from search_emails or get_email)."),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Reply body content. Plain text by default; set html=true for HTML."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the original email."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithBoolean("reply_all",
			mcp.Description("Reply to all original recipients (To + CC) instead of just the sender."),
			mcp.DefaultBool(false),
		),
		mcp.WithBoolean("html",
			mcp.Description("Set true if body contains HTML."),
			mcp.DefaultBool(false),
		),
	)
	s.AddTool(replyEmailTool, tools.ReplyEmailHandler(imapClient, smtpClient))

	// Register delete_email tool
	deleteEmailTool := mcp.NewTool("delete_email",
		mcp.WithDescription("Delete an email. By default moves to 'Deleted Messages' (trash). Set permanent=true for immediate removal. Use search_emails first to find email IDs."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID to delete (from search_emails)."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithBoolean("permanent",
			mcp.Description("Permanently expunge the email instead of moving to trash. This cannot be undone."),
			mcp.DefaultBool(false),
		),
	)
	s.AddTool(deleteEmailTool, tools.DeleteEmailHandler(imapClient))

	// Register move_email tool
	moveEmailTool := mcp.NewTool("move_email",
		mcp.WithDescription("Move an email from one folder to another. Use list_folders to discover valid folder names, and search_emails to find email IDs."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID to move (from search_emails)."),
		),
		mcp.WithString("from_folder",
			mcp.Description("Source mailbox folder."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithString("to_folder",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Destination mailbox folder (from list_folders)."),
		),
	)
	s.AddTool(moveEmailTool, tools.MoveEmailHandler(imapClient))

	// Register list_folders tool
	listFoldersTool := mcp.NewTool("list_folders",
		mcp.WithDescription("List all available mailbox folders. Returns folder names that can be used as the 'folder' parameter in other tools. Call this first to discover valid folder names."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
	)
	s.AddTool(listFoldersTool, tools.ListFoldersHandler(imapClient))

	// Register create_folder tool
	createFolderTool := mcp.NewTool("create_folder",
		mcp.WithDescription("Create a new mailbox folder. Optionally nest under a parent folder. Calling twice with the same name may fail if the folder already exists."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("name",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Name of the new folder."),
		),
		mcp.WithString("parent",
			mcp.Description("Parent folder path for nesting (from list_folders). Omit for top-level folder."),
		),
	)
	s.AddTool(createFolderTool, tools.CreateFolderHandler(imapClient))

	// Register delete_folder tool
	deleteFolderTool := mcp.NewTool("delete_folder",
		mcp.WithDescription("Delete a mailbox folder. Refuses if the folder contains emails unless force=true. Use list_folders to discover valid names."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("name",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Folder name to delete (from list_folders)."),
		),
		mcp.WithBoolean("force",
			mcp.Description("Delete even if the folder contains emails. All contained emails will be lost."),
			mcp.DefaultBool(false),
		),
	)
	s.AddTool(deleteFolderTool, tools.DeleteFolderHandler(imapClient))

	// Register mark_read tool
	markReadTool := mcp.NewTool("mark_read",
		mcp.WithDescription("Mark an email as read (seen) or unread (unseen). Use search_emails to find email IDs."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID to mark (from search_emails)."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithBoolean("read",
			mcp.Description("true to mark as read, false to mark as unread."),
			mcp.DefaultBool(true),
		),
	)
	s.AddTool(markReadTool, tools.MarkReadHandler(imapClient))

	// Register count_emails tool
	countEmailsTool := mcp.NewTool("count_emails",
		mcp.WithDescription("Count emails matching filters without fetching content. Lightweight alternative to search_emails when you only need a count."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder to count in."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithNumber("last_days",
			mcp.Description("Only count emails from the last N days."),
			mcp.Min(1),
		),
		mcp.WithBoolean("unread_only",
			mcp.Description("Only count unread (unseen) emails."),
			mcp.DefaultBool(false),
		),
	)
	s.AddTool(countEmailsTool, tools.CountEmailsHandler(imapClient))

	// Register draft_email tool
	draftEmailTool := mcp.NewTool("draft_email",
		mcp.WithDescription("Save an email as a draft in the Drafts folder for later review and sending. Returns a draft_id. Calling twice creates duplicate drafts."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithString("to",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Recipient email address (string) or JSON array of addresses."),
		),
		mcp.WithString("subject",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email subject line."),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email body content. Plain text by default; set html=true for HTML."),
		),
		mcp.WithString("cc",
			mcp.Description("CC email address (string) or JSON array of addresses."),
		),
		mcp.WithString("bcc",
			mcp.Description("BCC email address (string) or JSON array of addresses."),
		),
		mcp.WithBoolean("html",
			mcp.Description("Set true if body contains HTML."),
			mcp.DefaultBool(false),
		),
		mcp.WithString("reply_to_id",
			mcp.Description("Email UID of the original message if creating a reply draft. Sets In-Reply-To and References headers."),
		),
		mcp.WithString("folder",
			mcp.Description("Folder containing the original email for reply drafts."),
			mcp.DefaultString("INBOX"),
		),
	)
	s.AddTool(draftEmailTool, tools.DraftEmailHandler(imapClient, cfg.ICloudEmail))

	// Register get_attachment tool
	getAttachmentTool := mcp.NewTool("get_attachment",
		mcp.WithDescription("Download an email attachment by filename. Use get_email first to see available attachment filenames and sizes. Returns base64-encoded content by default, or saves to disk if save_path is provided."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID containing the attachment (from search_emails or get_email)."),
		),
		mcp.WithString("filename",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Exact filename of the attachment (from get_email attachments list)."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithString("save_path",
			mcp.Description("Absolute file path to save the attachment to disk. Must not contain '..'. If omitted, returns base64-encoded content in the response."),
		),
	)
	s.AddTool(getAttachmentTool, tools.GetAttachmentHandler(imapClient))

	// Register flag_email tool
	flagEmailTool := mcp.NewTool("flag_email",
		mcp.WithDescription("Set or remove flags on an email. Use 'none' to clear all flags. Use search_emails first to find email IDs."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.MinLength(1),
			mcp.Description("Email UID to flag (from search_emails)."),
		),
		mcp.WithString("flag",
			mcp.Required(),
			mcp.Enum("follow-up", "important", "deadline", "none"),
			mcp.Description("Flag type to set. Use 'none' to remove all flags."),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email."),
			mcp.DefaultString("INBOX"),
		),
		mcp.WithString("color",
			mcp.Enum("red", "orange", "yellow", "green", "blue", "purple"),
			mcp.Description("Optional flag color. Only applies when flag is not 'none'."),
		),
	)
	s.AddTool(flagEmailTool, tools.FlagEmailHandler(imapClient))

	// Log startup
	slog.Info("server starting",
		"version", version,
		"email", cfg.ICloudEmail,
		"imap_server", fmt.Sprintf("imap.mail.me.com:%d", 993),
		"smtp_server", fmt.Sprintf("smtp.mail.me.com:%d", 587),
	)

	// Start the stdio server with cancellable context
	stdioServer := server.NewStdioServer(s)
	if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// timeoutMiddleware wraps each tool handler with a context deadline.
func timeoutMiddleware(timeout time.Duration) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return next(ctx, req)
		}
	}
}

// loggingMiddleware logs each tool call with a unique request ID, tool name, duration, and outcome.
func loggingMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			requestID := uuid.New().String()
			tool := req.Params.Name
			logger := slog.With("request_id", requestID, "tool", tool)

			logger.Debug("tool call started")
			start := time.Now()

			result, err := next(ctx, req)
			duration := time.Since(start)

			if err != nil {
				logger.Error("tool call failed", "duration_ms", duration.Milliseconds(), "error", err)
			} else if result != nil && result.IsError {
				logger.Warn("tool call returned error", "duration_ms", duration.Milliseconds())
			} else {
				logger.Info("tool call completed", "duration_ms", duration.Milliseconds())
			}

			return result, err
		}
	}
}
