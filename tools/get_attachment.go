package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetAttachmentHandler creates a handler for downloading email attachments
func GetAttachmentHandler(imapClient EmailReader) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required email_id
		emailID, ok := args["email_id"].(string)
		if !ok || emailID == "" {
			return mcp.NewToolResultError("email_id is required"), nil
		}

		// Get required filename
		filename, ok := args["filename"].(string)
		if !ok || filename == "" {
			return mcp.NewToolResultError("filename is required"), nil
		}

		// Get folder (default to INBOX)
		folder, _ := args["folder"].(string)
		if folder == "" {
			folder = "INBOX"
		}

		// Get optional save_path
		savePath, _ := args["save_path"].(string)

		// Get attachment from IMAP
		attachment, err := imapClient.GetAttachment(ctx, folder, emailID, filename)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get attachment: %v", err)), nil
		}

		// Build response
		response := map[string]interface{}{
			"success":   true,
			"filename":  attachment.Filename,
			"size":      attachment.Size,
			"mime_type": attachment.MIMEType,
		}

		if savePath != "" {
			// Validate save path - check parent directory exists
			parentDir := filepath.Dir(savePath)
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				return mcp.NewToolResultError(fmt.Sprintf("save path directory does not exist: %s", parentDir)), nil
			}

			// Write file
			if err := os.WriteFile(savePath, attachment.Content, 0644); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to save attachment: %v", err)), nil
			}

			response["path"] = savePath
			response["saved"] = true
		} else {
			// Return base64 encoded content
			encoded := base64.StdEncoding.EncodeToString(attachment.Content)
			response["data"] = encoded
			response["saved"] = false
		}

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
