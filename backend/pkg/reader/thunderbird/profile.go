package thunderbird

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
)

// FindProfiles finds all Thunderbird profile directories on the system.
func FindProfiles() ([]string, error) {
	var profileDirs []string

	// Get base Thunderbird directory based on OS
	var baseDir string
	switch runtime.GOOS {
	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, "Library", "Thunderbird", "Profiles")
	case "linux":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, ".thunderbird")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil, fmt.Errorf("APPDATA environment variable not set")
		}
		baseDir = filepath.Join(appData, "Thunderbird", "Profiles")
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Check if base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("thunderbird directory not found: %s", baseDir)
	}

	// Find all profile directories
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("reading thunderbird directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			profilePath := filepath.Join(baseDir, entry.Name())
			profileDirs = append(profileDirs, profilePath)
		}
	}

	if len(profileDirs) == 0 {
		return nil, fmt.Errorf("no thunderbird profiles found in %s", baseDir)
	}

	return profileDirs, nil
}

// FindMailboxes finds the file paths for the specified mailboxes in a profile.
func FindMailboxes(profilePath string, mailboxNames []string) (map[string]string, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("profile path is empty")
	}

	// Verify profile path exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile path does not exist: %s", profilePath)
	}

	mailboxPaths := make(map[string]string)

	// Look for mailboxes in common locations
	mailDirs := []string{
		filepath.Join(profilePath, "Mail", "Local Folders"),
	}

	// Check for account-specific directories under Mail/
	mailDir := filepath.Join(profilePath, "Mail")
	if _, err := os.Stat(mailDir); err == nil {
		entries, err := os.ReadDir(mailDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					mailDirs = append(mailDirs, filepath.Join(mailDir, entry.Name()))
				}
			}
		}
	}

	// Check for IMAP account directories (e.g., ImapMail/imap.gmail.com/)
	imapDir := filepath.Join(profilePath, "ImapMail")
	if _, err := os.Stat(imapDir); err == nil {
		entries, err := os.ReadDir(imapDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					mailDirs = append(mailDirs, filepath.Join(imapDir, entry.Name()))
				}
			}
		}
	}

	logger := slog.Default()
	logger.Debug("searching for mailboxes in directories", "dirs", mailDirs)
	// Search for each requested mailbox
	for _, mailboxName := range mailboxNames {
		found := false

		for _, dir := range mailDirs {
			// Try direct path (e.g., "Inbox")
			mailboxPath := filepath.Join(dir, mailboxName)
			if _, err := os.Stat(mailboxPath); err == nil {
				mailboxPaths[mailboxName] = mailboxPath
				found = true
				break
			}

			// Try with .sbd extension for subdirectories
			mailboxPath = filepath.Join(dir, mailboxName+".sbd", mailboxName)
			if _, err := os.Stat(mailboxPath); err == nil {
				mailboxPaths[mailboxName] = mailboxPath
				found = true
				break
			}

			// Try lowercase (some systems use lowercase)
			mailboxPath = filepath.Join(dir, mailboxName)
			if _, err := os.Stat(mailboxPath); err == nil {
				mailboxPaths[mailboxName] = mailboxPath
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("mailbox not found: %s in profile %s", mailboxName, profilePath)
		}
	}

	return mailboxPaths, nil
}
