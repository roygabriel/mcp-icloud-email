package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// MarkReadHandler creates a handler for marking emails as read/unread
func MarkReadHandler(client EmailWriter) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Get read status (default to true)
		read := true
		if readArg, ok := args["read"].(bool); ok {
			read = readArg
		}

		// Mark email
		err := client.MarkRead(ctx, folder, emailID, read)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to mark email: %v", err)), nil
		}

		// Format response
		status := "read"
		if !read {
			status = "unread"
		}

		response := map[string]interface{}{
			"success": true,
			"email_id": emailID,
			"message": fmt.Sprintf("Email marked as %s successfully", status),
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
