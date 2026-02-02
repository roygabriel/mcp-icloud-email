package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-email/imap"
)

// MoveEmailHandler creates a handler for moving emails between folders
func MoveEmailHandler(client *imap.Client) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required parameters
		emailID, ok := args["email_id"].(string)
		if !ok || emailID == "" {
			return mcp.NewToolResultError("email_id is required"), nil
		}

		toFolder, ok := args["to_folder"].(string)
		if !ok || toFolder == "" {
			return mcp.NewToolResultError("to_folder is required"), nil
		}

		// Get from_folder (default to INBOX)
		fromFolder, _ := args["from_folder"].(string)
		if fromFolder == "" {
			fromFolder = "INBOX"
		}

		// Move email
		err := client.MoveEmail(ctx, fromFolder, toFolder, emailID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to move email: %v", err)), nil
		}

		// Format response
		response := map[string]interface{}{
			"success":     true,
			"email_id":    emailID,
			"from_folder": fromFolder,
			"to_folder":   toFolder,
			"message":     fmt.Sprintf("Email moved from '%s' to '%s' successfully", fromFolder, toFolder),
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
