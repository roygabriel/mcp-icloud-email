package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rgabriel/mcp-icloud-email/config"
	"github.com/rgabriel/mcp-icloud-email/imap"
	"github.com/rgabriel/mcp-icloud-email/smtp"
	"github.com/rgabriel/mcp-icloud-email/tools"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create IMAP client
	imapClient, err := imap.NewClient(cfg.ICloudEmail, cfg.ICloudPassword)
	if err != nil {
		log.Fatalf("Failed to create IMAP client: %v", err)
	}
	defer imapClient.Close()

	// Test IMAP connection by listing folders
	ctx := context.Background()
	_, err = imapClient.ListFolders(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to iCloud IMAP (check credentials): %v", err)
	}

	// Create SMTP client
	smtpClient := smtp.NewClient(cfg.ICloudEmail, cfg.ICloudPassword)

	// Create MCP server
	s := server.NewMCPServer(
		"iCloud Email Server",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// Register search_emails tool
	searchEmailsTool := mcp.NewTool("search_emails",
		mcp.WithDescription("Search and list emails with optional filters for date range, unread status, and text query"),
		mcp.WithString("query",
			mcp.Description("Optional search term to find in subject/body"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder to search in (default: INBOX)"),
		),
		mcp.WithNumber("last_days",
			mcp.Description("Only show emails from last N days (default: 30)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of emails to return (default: 50, max: 200)"),
		),
		mcp.WithBoolean("unread_only",
			mcp.Description("Only return unread emails (default: false)"),
		),
		mcp.WithString("since",
			mcp.Description("Optional start date filter in ISO 8601 format (e.g., '2024-01-15T14:30:00Z')"),
		),
		mcp.WithString("before",
			mcp.Description("Optional end date filter in ISO 8601 format (e.g., '2024-01-15T14:30:00Z')"),
		),
	)
	s.AddTool(searchEmailsTool, tools.SearchEmailsHandler(imapClient))

	// Register get_email tool
	getEmailTool := mcp.NewTool("get_email",
		mcp.WithDescription("Get full email content including body, headers, and attachments list"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) to retrieve"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email (default: INBOX)"),
		),
	)
	s.AddTool(getEmailTool, tools.GetEmailHandler(imapClient))

	// Register send_email tool
	sendEmailTool := mcp.NewTool("send_email",
		mcp.WithDescription("Send a new email"),
		mcp.WithString("to",
			mcp.Required(),
			mcp.Description("Recipient email address or array of addresses"),
		),
		mcp.WithString("subject",
			mcp.Required(),
			mcp.Description("Email subject line"),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.Description("Email body content"),
		),
		mcp.WithString("cc",
			mcp.Description("Optional CC email address or array of addresses"),
		),
		mcp.WithString("bcc",
			mcp.Description("Optional BCC email address or array of addresses"),
		),
		mcp.WithBoolean("html",
			mcp.Description("Whether body is HTML format (default: false)"),
		),
	)
	s.AddTool(sendEmailTool, tools.SendEmailHandler(smtpClient, cfg.ICloudEmail))

	// Register reply_email tool
	replyEmailTool := mcp.NewTool("reply_email",
		mcp.WithDescription("Reply to an existing email"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) being replied to"),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.Description("Reply message body"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the original email (default: INBOX)"),
		),
		mcp.WithBoolean("reply_all",
			mcp.Description("Reply to all recipients (default: false)"),
		),
		mcp.WithBoolean("html",
			mcp.Description("Whether body is HTML format (default: false)"),
		),
	)
	s.AddTool(replyEmailTool, tools.ReplyEmailHandler(imapClient, smtpClient))

	// Register delete_email tool
	deleteEmailTool := mcp.NewTool("delete_email",
		mcp.WithDescription("Delete an email (move to trash or permanently delete)"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) to delete"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email (default: INBOX)"),
		),
		mcp.WithBoolean("permanent",
			mcp.Description("Permanently delete instead of moving to trash (default: false)"),
		),
	)
	s.AddTool(deleteEmailTool, tools.DeleteEmailHandler(imapClient))

	// Register move_email tool
	moveEmailTool := mcp.NewTool("move_email",
		mcp.WithDescription("Move an email from one folder to another"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) to move"),
		),
		mcp.WithString("from_folder",
			mcp.Description("Source mailbox folder (default: INBOX)"),
		),
		mcp.WithString("to_folder",
			mcp.Required(),
			mcp.Description("Destination mailbox folder"),
		),
	)
	s.AddTool(moveEmailTool, tools.MoveEmailHandler(imapClient))

	// Register list_folders tool
	listFoldersTool := mcp.NewTool("list_folders",
		mcp.WithDescription("List all available mailbox folders"),
	)
	s.AddTool(listFoldersTool, tools.ListFoldersHandler(imapClient))

	// Register mark_read tool
	markReadTool := mcp.NewTool("mark_read",
		mcp.WithDescription("Mark an email as read or unread"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) to mark"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email (default: INBOX)"),
		),
		mcp.WithBoolean("read",
			mcp.Description("Mark as read (true) or unread (false) (default: true)"),
		),
	)
	s.AddTool(markReadTool, tools.MarkReadHandler(imapClient))

	// Register count_emails tool
	countEmailsTool := mcp.NewTool("count_emails",
		mcp.WithDescription("Count emails matching filters without fetching full content"),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder to count in (default: INBOX)"),
		),
		mcp.WithNumber("last_days",
			mcp.Description("Only count emails from last N days"),
		),
		mcp.WithBoolean("unread_only",
			mcp.Description("Only count unread emails (default: false)"),
		),
	)
	s.AddTool(countEmailsTool, tools.CountEmailsHandler(imapClient))

	// Register draft_email tool
	draftEmailTool := mcp.NewTool("draft_email",
		mcp.WithDescription("Save email as draft for later review and sending"),
		mcp.WithString("to",
			mcp.Required(),
			mcp.Description("Recipient email address or array of addresses"),
		),
		mcp.WithString("subject",
			mcp.Required(),
			mcp.Description("Email subject line"),
		),
		mcp.WithString("body",
			mcp.Required(),
			mcp.Description("Email body content"),
		),
		mcp.WithString("cc",
			mcp.Description("Optional CC email address or array of addresses"),
		),
		mcp.WithString("bcc",
			mcp.Description("Optional BCC email address or array of addresses"),
		),
		mcp.WithBoolean("html",
			mcp.Description("Whether body is HTML format (default: false)"),
		),
		mcp.WithString("reply_to_id",
			mcp.Description("Original email ID if this is a reply draft"),
		),
		mcp.WithString("folder",
			mcp.Description("Folder containing original email for reply (default: INBOX)"),
		),
	)
	s.AddTool(draftEmailTool, tools.DraftEmailHandler(imapClient, cfg.ICloudEmail))

	// Register get_attachment tool
	getAttachmentTool := mcp.NewTool("get_attachment",
		mcp.WithDescription("Download email attachment by filename"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) containing the attachment"),
		),
		mcp.WithString("filename",
			mcp.Required(),
			mcp.Description("Name of attachment to download"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email (default: INBOX)"),
		),
		mcp.WithString("save_path",
			mcp.Description("Path to save file (returns base64 if omitted)"),
		),
	)
	s.AddTool(getAttachmentTool, tools.GetAttachmentHandler(imapClient))

	// Register flag_email tool
	flagEmailTool := mcp.NewTool("flag_email",
		mcp.WithDescription("Flag email for follow-up with optional color"),
		mcp.WithString("email_id",
			mcp.Required(),
			mcp.Description("Email ID (UID) to flag"),
		),
		mcp.WithString("flag",
			mcp.Required(),
			mcp.Description("Flag type: follow-up, important, deadline, none"),
		),
		mcp.WithString("folder",
			mcp.Description("Mailbox folder containing the email (default: INBOX)"),
		),
		mcp.WithString("color",
			mcp.Description("Flag color: red, orange, yellow, green, blue, purple"),
		),
	)
	s.AddTool(flagEmailTool, tools.FlagEmailHandler(imapClient))

	// Log startup
	fmt.Fprintf(os.Stderr, "iCloud Email MCP Server v1.0.0 starting...\n")
	fmt.Fprintf(os.Stderr, "Connected to iCloud as: %s\n", cfg.ICloudEmail)
	fmt.Fprintf(os.Stderr, "IMAP: %s@%s:%d\n", cfg.ICloudEmail, "imap.mail.me.com", 993)
	fmt.Fprintf(os.Stderr, "SMTP: %s@%s:%d\n", cfg.ICloudEmail, "smtp.mail.me.com", 587)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
