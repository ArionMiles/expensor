package httpapi

import "strings"

const defaultAvatarKey = "default"

var validAvatarKeys = map[string]struct{}{
	defaultAvatarKey: {},
	"ledger":         {},
	"wallet":         {},
}

// ValidAvatarKey reports whether key identifies a bundled account avatar.
func ValidAvatarKey(key string) bool {
	_, ok := validAvatarKeys[strings.TrimSpace(key)]
	return ok
}

func normalizeAvatarKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return defaultAvatarKey
	}
	return trimmed
}
