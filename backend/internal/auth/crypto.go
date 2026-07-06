package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// SecretKeySize is the required key length for AES-256-GCM.
const SecretKeySize = 32

// SecretAssociatedData binds ciphertext to a tenant, subject, and credential kind.
type SecretAssociatedData struct {
	TenantID string
	Scope    string
	Name     string
	Kind     string
}

// SecretBox seals and opens secrets using authenticated encryption.
type SecretBox struct {
	aead cipher.AEAD
}

// NewSecretBox creates a SecretBox from a 32-byte key.
func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != SecretKeySize {
		return nil, fmt.Errorf("secret key must be %d bytes", SecretKeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

// Seal encrypts plaintext and authenticates associated data.
func (b *SecretBox) Seal(plaintext []byte, associated SecretAssociatedData) ([]byte, error) {
	if b == nil || b.aead == nil {
		return nil, errors.New("secret box is not initialized")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	out := make([]byte, 0, len(nonce)+len(plaintext)+b.aead.Overhead())
	out = append(out, nonce...)
	out = b.aead.Seal(out, nonce, plaintext, associated.bytes())
	return out, nil
}

// Open decrypts ciphertext only when associated data matches.
func (b *SecretBox) Open(ciphertext []byte, associated SecretAssociatedData) ([]byte, error) {
	if b == nil || b.aead == nil {
		return nil, errors.New("secret box is not initialized")
	}
	nonceSize := b.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := b.aead.Open(nil, nonce, sealed, associated.bytes())
	if err != nil {
		return nil, fmt.Errorf("decrypting secret: %w", err)
	}
	return plaintext, nil
}

func (a SecretAssociatedData) bytes() []byte {
	return []byte(a.TenantID + "\x00" + a.Scope + "\x00" + a.Name + "\x00" + a.Kind)
}
