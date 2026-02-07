package tools

import (
	"strings"
	"testing"
)

func TestValidateSavePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{name: "empty is ok", path: ""},
		{name: "absolute path", path: "/home/user/downloads/file.pdf"},
		{name: "traversal rejected", path: "/home/user/../etc/passwd", wantErr: true, errMsg: "traversal"},
		{name: "null byte rejected", path: "/home/user/file\x00.pdf", wantErr: true, errMsg: "null"},
		{name: "relative path rejected", path: "relative/path.pdf", wantErr: true, errMsg: "absolute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSavePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateFolderName(t *testing.T) {
	tests := []struct {
		name    string
		folder  string
		wantErr bool
		errMsg  string
	}{
		{name: "valid name", folder: "Projects"},
		{name: "valid nested", folder: "Work"},
		{name: "empty rejected", folder: "", wantErr: true, errMsg: "empty"},
		{name: "traversal rejected", folder: "..", wantErr: true, errMsg: ".."},
		{name: "traversal in path", folder: "foo/../bar", wantErr: true, errMsg: ".."},
		{name: "null byte rejected", folder: "test\x00folder", wantErr: true, errMsg: "null"},
		{name: "wildcard * rejected", folder: "test*", wantErr: true, errMsg: "wildcard"},
		{name: "wildcard % rejected", folder: "test%", wantErr: true, errMsg: "wildcard"},
		{name: "newline rejected", folder: "test\nfolder", wantErr: true, errMsg: "control"},
		{name: "tab rejected", folder: "test\tfolder", wantErr: true, errMsg: "control"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFolderName(tt.folder)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateEmailID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid numeric", id: "12345"},
		{name: "empty rejected", id: "", wantErr: true},
		{name: "null byte rejected", id: "123\x00", wantErr: true},
		{name: "control char rejected", id: "123\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmailID(tt.id)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		wantErr bool
		errMsg  string
	}{
		{name: "valid", file: "document.pdf"},
		{name: "empty rejected", file: "", wantErr: true},
		{name: "slash rejected", file: "path/file.pdf", wantErr: true, errMsg: "separator"},
		{name: "backslash rejected", file: "path\\file.pdf", wantErr: true, errMsg: "separator"},
		{name: "double dot rejected", file: "file..pdf", wantErr: true, errMsg: ".."},
		{name: "null rejected", file: "file\x00.pdf", wantErr: true, errMsg: "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilename(tt.file)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errMsg != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errMsg) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBodySize(t *testing.T) {
	// Normal body should pass
	if err := validateBodySize("hello world"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Oversized body should fail
	huge := strings.Repeat("x", maxBodySize+1)
	if err := validateBodySize(huge); err == nil {
		t.Fatal("expected error for oversized body")
	}
}

func TestValidateSubjectSize(t *testing.T) {
	// Normal subject should pass
	if err := validateSubjectSize("Hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Oversized subject should fail
	huge := strings.Repeat("x", maxSubjectSize+1)
	if err := validateSubjectSize(huge); err == nil {
		t.Fatal("expected error for oversized subject")
	}
}
