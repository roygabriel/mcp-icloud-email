package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// ListFoldersHandler creates a handler for listing available folders
func ListFoldersHandler(client EmailReader) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// List folders
		folders, err := client.ListFolders(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list folders: %v", err)), nil
		}

		// Format response
		response := map[string]interface{}{
			"count":   len(folders),
			"folders": folders,
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
