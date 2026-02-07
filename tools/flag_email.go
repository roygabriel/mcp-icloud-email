package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// FlagEmailHandler creates a handler for flagging emails
func FlagEmailHandler(imapClient EmailWriter) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// Get required email_id
		emailID, ok := args["email_id"].(string)
		if !ok || emailID == "" {
			return mcp.NewToolResultError("email_id is required"), nil
		}

		// Get required flag type
		flagType, ok := args["flag"].(string)
		if !ok || flagType == "" {
			return mcp.NewToolResultError("flag is required"), nil
		}

		// Validate flag type
		validFlags := map[string]bool{
			"follow-up": true,
			"important": true,
			"deadline":  true,
			"none":      true,
		}
		if !validFlags[flagType] {
			return mcp.NewToolResultError("flag must be one of: follow-up, important, deadline, none"), nil
		}

		// Get folder (default to INBOX)
		folder, _ := args["folder"].(string)
		if folder == "" {
			folder = "INBOX"
		}

		// Get optional color
		color, _ := args["color"].(string)
		if color != "" {
			// Validate color
			validColors := map[string]bool{
				"red":    true,
				"orange": true,
				"yellow": true,
				"green":  true,
				"blue":   true,
				"purple": true,
			}
			if !validColors[color] {
				return mcp.NewToolResultError("color must be one of: red, orange, yellow, green, blue, purple"), nil
			}
		}

		// Flag the email
		err := imapClient.FlagEmail(ctx, folder, emailID, flagType, color)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to flag email: %v", err)), nil
		}

		// Format response
		response := map[string]interface{}{
			"success":  true,
			"email_id": emailID,
			"flag":     flagType,
		}

		if color != "" {
			response["color"] = color
		}

		var message string
		if flagType == "none" {
			message = "Email flags removed successfully"
		} else {
			if color != "" {
				message = fmt.Sprintf("Email flagged as %s (%s) successfully", flagType, color)
			} else {
				message = fmt.Sprintf("Email flagged as %s successfully", flagType)
			}
		}
		response["message"] = message

		jsonData, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}
