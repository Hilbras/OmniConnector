package credential

import (
	"testing"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	store, err := NewStore(key)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	material := &contracts.CredentialMaterial{
		Data: map[string]string{
			"token":   "secret-token-value",
			"refresh": "refresh-token-value",
		},
	}

	encrypted, err := store.Encrypt(material)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted value is empty")
	}

	decrypted, err := store.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted.Data["token"] != "secret-token-value" {
		t.Errorf("token mismatch: got %s", decrypted.Data["token"])
	}
	if decrypted.Data["refresh"] != "refresh-token-value" {
		t.Errorf("refresh mismatch: got %s", decrypted.Data["refresh"])
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := NewStore(make([]byte, 16))
	if err == nil {
		t.Error("expected error for invalid key length")
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	key := make([]byte, 32)
	store, _ := NewStore(key)
	_, err := store.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid ciphertext")
	}
}
