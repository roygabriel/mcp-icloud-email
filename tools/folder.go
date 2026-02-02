package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rgabriel/mcp-icloud-email/imap"
)

// CreateFolderHandler creates a handler for creating a new folder
func CreateFolderHandler(client *imap.Client) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get folder name (required)
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return mcp.NewToolResultError("name parameter is required"), nil
		}

		// Get parent folder (optional)
		parent, _ := args["parent"].(string)

		// Construct full folder path
		folderPath := name
		if parent != "" {
			folderPath = parent + "/" + name
		}

		// Create the folder
		if err := client.CreateFolder(ctx, name, parent); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create folder: %v", err)), nil
		}

		// Format response
		response := map[string]interface{}{
			"success":     true,
			"folder_name": name,
			"path":        folderPath,
			"message":     fmt.Sprintf("Folder '%s' created successfully", folderPath),
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}

// DeleteFolderHandler creates a handler for deleting a folder
func DeleteFolderHandler(client *imap.Client) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get folder name (required)
		name, ok := args["name"].(string)
		if !ok || name == "" {
			return mcp.NewToolResultError("name parameter is required"), nil
		}

		// Get force flag (optional, default false)
		force := false
		if forceArg, ok := args["force"].(bool); ok {
			force = forceArg
		}

		// Delete the folder
		wasEmpty, emailCount, err := client.DeleteFolder(ctx, name, force)
		if err != nil {
			// Check if this is a "not empty" error
			if !force && emailCount > 0 {
				// Return a structured error response for non-empty folders
				response := map[string]interface{}{
					"success":     false,
					"folder_name": name,
					"email_count": emailCount,
					"message":     fmt.Sprintf("Folder '%s' is not empty (contains %d emails). Use force=true to delete anyway.", name, emailCount),
				}
				jsonData, _ := json.MarshalIndent(response, "", "  ")
				return mcp.NewToolResultText(string(jsonData)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to delete folder: %v", err)), nil
		}

		// Format success response
		response := map[string]interface{}{
			"success":     true,
			"folder_name": name,
			"was_empty":   wasEmpty,
			"message":     fmt.Sprintf("Folder '%s' deleted successfully", name),
		}

		if !wasEmpty {
			response["emails_deleted"] = emailCount
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
