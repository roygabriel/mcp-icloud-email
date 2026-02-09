package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  string
	}{
		{
			name:     "both vars set",
			email:    "user@icloud.com",
			password: "app-specific-password",
		},
		{
			name:     "missing email",
			email:    "",
			password: "app-specific-password",
			wantErr:  "ICLOUD_EMAIL environment variable is required",
		},
		{
			name:     "missing password",
			email:    "user@icloud.com",
			password: "",
			wantErr:  "ICLOUD_PASSWORD environment variable is required (use app-specific password from appleid.apple.com)",
		},
		{
			name:     "both missing",
			email:    "",
			password: "",
			wantErr:  "ICLOUD_EMAIL environment variable is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear any .env influence by setting explicit values
			if tt.email != "" {
				t.Setenv("ICLOUD_EMAIL", tt.email)
			} else {
				t.Setenv("ICLOUD_EMAIL", "")
			}
			if tt.password != "" {
				t.Setenv("ICLOUD_PASSWORD", tt.password)
			} else {
				t.Setenv("ICLOUD_PASSWORD", "")
			}

			cfg, err := Load()

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				if cfg != nil {
					t.Fatal("expected nil config on error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.ICloudEmail != tt.email {
				t.Errorf("ICloudEmail = %q, want %q", cfg.ICloudEmail, tt.email)
			}
			if cfg.ICloudPassword != tt.password {
				t.Errorf("ICloudPassword = %q, want %q", cfg.ICloudPassword, tt.password)
			}
		})
	}
}
