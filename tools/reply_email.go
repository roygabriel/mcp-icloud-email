package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-email/imap"
	"github.com/rgabriel/mcp-icloud-email/smtp"
)

// ReplyEmailHandler creates a handler for replying to emails
func ReplyEmailHandler(imapClient *imap.Client, smtpClient *smtp.Client) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required parameters
		emailID, ok := args["email_id"].(string)
		if !ok || emailID == "" {
			return mcp.NewToolResultError("email_id is required"), nil
		}

		body, ok := args["body"].(string)
		if !ok || body == "" {
			return mcp.NewToolResultError("body is required"), nil
		}

		// Get optional parameters
		folder, _ := args["folder"].(string)
		if folder == "" {
			folder = "INBOX"
		}

		replyAll := false
		if ra, ok := args["reply_all"].(bool); ok {
			replyAll = ra
		}

		html := false
		if h, ok := args["html"].(bool); ok {
			html = h
		}

		// Fetch the original email
		originalEmail, err := imapClient.GetEmail(ctx, folder, emailID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get original email: %v", err)), nil
		}

		// Build send options
		opts := smtp.SendOptions{
			HTML: html,
		}

		// Reply to the email
		err = smtpClient.ReplyToEmail(ctx, originalEmail, body, replyAll, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send reply: %v", err)), nil
		}

		// Format response
		replyType := "Reply"
		if replyAll {
			replyType = "Reply All"
		}

		response := map[string]interface{}{
			"success":       true,
			"message":       fmt.Sprintf("%s sent successfully", replyType),
			"original_subject": originalEmail.Subject,
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
