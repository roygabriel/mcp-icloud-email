package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maxBodySize    = 10 * 1024 * 1024 // 10 MB
	maxSubjectSize = 998              // RFC 2822 line length limit
)

// validateSavePath rejects paths that could escape intended directories.
func validateSavePath(path string) error {
	if path == "" {
		return nil
	}

	// Reject null bytes
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("save_path must not contain null bytes")
	}

	// Reject raw traversal sequences before cleaning
	if strings.Contains(path, "..") {
		return fmt.Errorf("save_path must not contain path traversal (..)")
	}

	// Must be absolute
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("save_path must be an absolute path")
	}

	return nil
}

// validateFolderName rejects folder names with dangerous characters.
func validateFolderName(name string) error {
	if name == "" {
		return fmt.Errorf("folder name must not be empty")
	}

	// Reject null bytes
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("folder name must not contain null bytes")
	}

	// Reject path traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("folder name must not contain '..'")
	}

	// Reject IMAP wildcards
	if strings.ContainsAny(name, "*%") {
		return fmt.Errorf("folder name must not contain wildcards (* or %%)")
	}

	// Reject newlines and control characters
	for _, r := range name {
		if r < 0x20 {
			return fmt.Errorf("folder name must not contain control characters")
		}
	}

	return nil
}

// validateEmailID checks that an email ID looks like a numeric UID.
func validateEmailID(id string) error {
	if id == "" {
		return fmt.Errorf("email_id is required")
	}

	// Reject null bytes and control characters
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("email_id contains invalid characters")
		}
	}

	return nil
}

// validateBodySize checks that body content doesn't exceed limits.
func validateBodySize(body string) error {
	if len(body) > maxBodySize {
		return fmt.Errorf("body exceeds maximum size of %d bytes", maxBodySize)
	}
	return nil
}

// validateSubjectSize checks that subject doesn't exceed limits.
func validateSubjectSize(subject string) error {
	if len(subject) > maxSubjectSize {
		return fmt.Errorf("subject exceeds maximum length of %d characters", maxSubjectSize)
	}
	return nil
}

// validateFilename rejects filenames with path traversal characters.
func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename is required")
	}

	// Reject null bytes
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("filename must not contain null bytes")
	}

	// Reject path separators and traversal
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("filename must not contain path separators")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("filename must not contain '..'")
	}

	return nil
}
