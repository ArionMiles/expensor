package auth_test

import (
	"bytes"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/auth"
)

func TestSealOpenBindsAssociatedData(t *testing.T) {
	key := bytes.Repeat([]byte{7}, auth.SecretKeySize)
	box, err := auth.NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox() error = %v", err)
	}
	associated := auth.SecretAssociatedData{TenantID: "tenant-a", Scope: "reader", Name: "gmail", Kind: "oauth_token"}
	ciphertext, err := box.Seal([]byte(`{"access_token":"secret"}`), associated)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	if bytes.Contains(ciphertext, []byte("access_token")) {
		t.Fatal("ciphertext contains plaintext")
	}
	if _, err := box.Open(ciphertext, auth.SecretAssociatedData{TenantID: "tenant-b", Scope: "reader", Name: "gmail", Kind: "oauth_token"}); err == nil {
		t.Fatal("Open() succeeded with wrong tenant associated data")
	}
}
