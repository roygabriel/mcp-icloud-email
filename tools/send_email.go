package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-email/smtp"
)

// SendEmailHandler creates a handler for sending emails
func SendEmailHandler(smtpClient *smtp.Client, fromEmail string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required parameters
		subject, ok := args["subject"].(string)
		if !ok || subject == "" {
			return mcp.NewToolResultError("subject is required"), nil
		}

		body, ok := args["body"].(string)
		if !ok || body == "" {
			return mcp.NewToolResultError("body is required"), nil
		}

		// Parse To addresses (can be string or array)
		var to []string
		switch v := args["to"].(type) {
		case string:
			if v == "" {
				return mcp.NewToolResultError("to is required"), nil
			}
			to = []string{v}
		case []interface{}:
			if len(v) == 0 {
				return mcp.NewToolResultError("to is required"), nil
			}
			for _, addr := range v {
				if str, ok := addr.(string); ok && str != "" {
					to = append(to, str)
				}
			}
		default:
			return mcp.NewToolResultError("to must be a string or array of strings"), nil
		}

		if len(to) == 0 {
			return mcp.NewToolResultError("to is required"), nil
		}

		// Validate email addresses
		for _, addr := range to {
			if _, err := mail.ParseAddress(addr); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid email address '%s': %v", addr, err)), nil
			}
		}

		// Build send options
		opts := smtp.SendOptions{}

		// Parse CC addresses
		if ccArg, ok := args["cc"]; ok && ccArg != nil {
			switch v := ccArg.(type) {
			case string:
				if v != "" {
					opts.CC = []string{v}
				}
			case []interface{}:
				for _, addr := range v {
					if str, ok := addr.(string); ok && str != "" {
						opts.CC = append(opts.CC, str)
					}
				}
			}
		}

		// Validate CC addresses
		for _, addr := range opts.CC {
			if _, err := mail.ParseAddress(addr); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid CC email address '%s': %v", addr, err)), nil
			}
		}

		// Parse BCC addresses
		if bccArg, ok := args["bcc"]; ok && bccArg != nil {
			switch v := bccArg.(type) {
			case string:
				if v != "" {
					opts.BCC = []string{v}
				}
			case []interface{}:
				for _, addr := range v {
					if str, ok := addr.(string); ok && str != "" {
						opts.BCC = append(opts.BCC, str)
					}
				}
			}
		}

		// Validate BCC addresses
		for _, addr := range opts.BCC {
			if _, err := mail.ParseAddress(addr); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid BCC email address '%s': %v", addr, err)), nil
			}
		}

		// Parse HTML flag
		if html, ok := args["html"].(bool); ok {
			opts.HTML = html
		}

		// Send email
		err := smtpClient.SendEmail(ctx, fromEmail, to, subject, body, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send email: %v", err)), nil
		}

		// Format response
		response := map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Email sent successfully to %v", to),
			"subject": subject,
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
