package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const opaqueTokenBytes = 32

// NewOpaqueToken creates a random token with prefix and returns the raw token plus its stable hash.
func NewOpaqueToken(prefix string) (raw, hash string, err error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", "", fmt.Errorf("token prefix cannot be blank")
	}

	random := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", "", fmt.Errorf("generating token: %w", err)
	}

	raw = prefix + "_" + base64.RawURLEncoding.EncodeToString(random)
	return raw, HashOpaqueToken(raw), nil
}

// HashOpaqueToken returns a SHA-256 hash suitable for storing opaque token lookups.
func HashOpaqueToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}
