package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetEmailHandler creates a handler for getting full email content
func GetEmailHandler(client EmailReader) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required email_id
		emailID, ok := args["email_id"].(string)
		if !ok || emailID == "" {
			return mcp.NewToolResultError("email_id is required"), nil
		}

		// Get folder (default to INBOX)
		folder, _ := args["folder"].(string)
		if folder == "" {
			folder = "INBOX"
		}

		// Get full email
		email, err := client.GetEmail(ctx, folder, emailID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get email: %v", err)), nil
		}

		// Format response
		jsonData, err := json.MarshalIndent(email, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
