package thunderbird

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	if _, err := os.Stat(baseDir); os.IsNotExist(err) { //nolint:gosec // G703: path from OS-determined Thunderbird directory, not user HTTP input
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
	mailDirs := collectMailDirs(profilePath)

	slog.Default().Debug("searching for mailboxes in directories", "dirs", mailDirs)

	for _, mailboxName := range mailboxNames {
		path, found := findMailboxInDirs(mailboxName, mailDirs)
		if !found {
			return nil, fmt.Errorf("mailbox not found: %s in profile %s", mailboxName, profilePath)
		}
		mailboxPaths[mailboxName] = path
	}

	return mailboxPaths, nil
}

// collectMailDirs returns all mail directories to search within a profile.
func collectMailDirs(profilePath string) []string {
	dirs := []string{filepath.Join(profilePath, "Mail", "Local Folders")}

	for _, subDir := range []string{"Mail", "ImapMail"} {
		dir := filepath.Join(profilePath, subDir)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, filepath.Join(dir, entry.Name()))
			}
		}
	}

	return dirs
}

// findMailboxInDirs searches for a mailbox file by name across a list of directories.
// It tries a direct path and an .sbd subdirectory path.
func findMailboxInDirs(mailboxName string, dirs []string) (string, bool) {
	for _, dir := range dirs {
		if path := filepath.Join(dir, mailboxName); pathExists(path) {
			return path, true
		}
		if path := filepath.Join(dir, mailboxName+".sbd", mailboxName); pathExists(path) {
			return path, true
		}
	}
	return "", false
}

// pathExists returns true if the path exists on the filesystem.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ListMailboxes returns the names of all available mailboxes in a Thunderbird profile.
// It walks Mail/Local Folders, Mail/<account>/, and ImapMail/<account>/ directories,
// returning file names that are MBOX files (no file extension — .msf index files are excluded).
// Results are deduplicated and sorted alphabetically.
func ListMailboxes(profilePath string) ([]string, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("profile path is empty")
	}
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile path does not exist: %s", profilePath)
	}

	dirs := collectMailDirs(profilePath)
	seen := make(map[string]struct{})
	var mailboxes []string

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if filepath.Ext(name) != "" {
				continue
			}
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				mailboxes = append(mailboxes, name)
			}
		}
	}

	sort.Strings(mailboxes)
	return mailboxes, nil
}
