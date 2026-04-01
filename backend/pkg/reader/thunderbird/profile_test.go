package thunderbird

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFindProfiles(t *testing.T) {
	// This test is platform-dependent, so we'll create a mock setup
	// In a real scenario, we'd need actual Thunderbird profiles

	t.Run("no thunderbird directory", func(t *testing.T) {
		// This test would fail on systems with Thunderbird installed
		// Skip it if Thunderbird exists
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot get home directory")
		}

		var tbDir string
		switch runtime.GOOS {
		case "darwin":
			tbDir = filepath.Join(homeDir, "Library", "Thunderbird", "Profiles")
		case "linux":
			tbDir = filepath.Join(homeDir, ".thunderbird")
		case "windows":
			appData := os.Getenv("APPDATA")
			if appData != "" {
				tbDir = filepath.Join(appData, "Thunderbird", "Profiles")
			}
		}

		if tbDir != "" {
			if _, err := os.Stat(tbDir); err == nil {
				t.Skip("Thunderbird directory exists, skipping test")
			}
		}

		_, err = FindProfiles()
		if err == nil {
			t.Error("expected error when Thunderbird directory doesn't exist")
		}
	})
}

func TestFindMailboxes(t *testing.T) {
	tests := []struct {
		name        string
		mailboxes   []string
		setupFunc   func(t *testing.T) string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid mailboxes in Local Folders",
			mailboxes: []string{"Inbox", "Sent"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				localFolders := filepath.Join(tmpDir, "Mail", "Local Folders")
				if err := os.MkdirAll(localFolders, 0o755); err != nil {
					t.Fatalf("failed to create local folders: %v", err)
				}

				// Create mailbox files
				for _, mb := range []string{"Inbox", "Sent"} {
					path := filepath.Join(localFolders, mb)
					if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
						t.Fatalf("failed to create mailbox %s: %v", mb, err)
					}
				}

				return tmpDir
			},
			wantErr: false,
		},
		{
			name:      "mailbox not found",
			mailboxes: []string{"NonExistent"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				localFolders := filepath.Join(tmpDir, "Mail", "Local Folders")
				if err := os.MkdirAll(localFolders, 0o755); err != nil {
					t.Fatalf("failed to create local folders: %v", err)
				}
				return tmpDir
			},
			wantErr:     true,
			errContains: "mailbox not found",
		},
		{
			name:      "empty profile path",
			mailboxes: []string{"Inbox"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				return ""
			},
			wantErr:     true,
			errContains: "profile path is empty",
		},
		{
			name:      "profile path does not exist",
			mailboxes: []string{"Inbox"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				return "/nonexistent/path"
			},
			wantErr:     true,
			errContains: "profile path does not exist",
		},
		{
			name:      "mailboxes in subdirectory with .sbd",
			mailboxes: []string{"Archive"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				localFolders := filepath.Join(tmpDir, "Mail", "Local Folders")
				archiveDir := filepath.Join(localFolders, "Archive.sbd")
				if err := os.MkdirAll(archiveDir, 0o755); err != nil {
					t.Fatalf("failed to create archive dir: %v", err)
				}

				archivePath := filepath.Join(archiveDir, "Archive")
				if err := os.WriteFile(archivePath, []byte(""), 0o600); err != nil {
					t.Fatalf("failed to create archive mailbox: %v", err)
				}

				return tmpDir
			},
			wantErr: false,
		},
		{
			name:      "mailboxes in ImapMail directory",
			mailboxes: []string{"INBOX"},
			setupFunc: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				// Create Mail directory with account subdirectory (common in Thunderbird)
				imapDir := filepath.Join(tmpDir, "Mail", "imap.gmail.com")
				if err := os.MkdirAll(imapDir, 0o755); err != nil {
					t.Fatalf("failed to create imap dir: %v", err)
				}

				inboxPath := filepath.Join(imapDir, "INBOX")
				if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
					t.Fatalf("failed to create inbox: %v", err)
				}

				return tmpDir
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profilePath := tt.setupFunc(t)

			mailboxPaths, err := FindMailboxes(profilePath, tt.mailboxes)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if len(mailboxPaths) != len(tt.mailboxes) {
					t.Errorf("expected %d mailboxes, got %d", len(tt.mailboxes), len(mailboxPaths))
				}

				for _, mb := range tt.mailboxes {
					if _, exists := mailboxPaths[mb]; !exists {
						t.Errorf("mailbox %q not found in results", mb)
					}
				}

				// Verify paths exist
				for mb, path := range mailboxPaths {
					if _, err := os.Stat(path); os.IsNotExist(err) {
						t.Errorf("mailbox path for %q does not exist: %s", mb, path)
					}
				}
			}
		})
	}
}

func TestFindMailboxes_MultipleAccounts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple mail directories
	localFolders := filepath.Join(tmpDir, "Mail", "Local Folders")
	imapDir := filepath.Join(tmpDir, "Mail", "imap.example.com")

	for _, dir := range []string{localFolders, imapDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create Inbox in local folders
	inboxPath := filepath.Join(localFolders, "Inbox")
	if err := os.WriteFile(inboxPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}

	// Create Sent in imap directory
	sentPath := filepath.Join(imapDir, "Sent")
	if err := os.WriteFile(sentPath, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create sent: %v", err)
	}

	mailboxPaths, err := FindMailboxes(tmpDir, []string{"Inbox", "Sent"})
	if err != nil {
		t.Fatalf("FindMailboxes failed: %v", err)
	}

	if len(mailboxPaths) != 2 {
		t.Errorf("expected 2 mailboxes, got %d", len(mailboxPaths))
	}

	// Verify both were found
	if _, exists := mailboxPaths["Inbox"]; !exists {
		t.Error("Inbox not found")
	}
	if _, exists := mailboxPaths["Sent"]; !exists {
		t.Error("Sent not found")
	}
}

func TestFindMailboxes_EmptyMailboxList(t *testing.T) {
	tmpDir := t.TempDir()

	mailboxPaths, err := FindMailboxes(tmpDir, []string{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(mailboxPaths) != 0 {
		t.Errorf("expected empty map, got %d mailboxes", len(mailboxPaths))
	}
}
