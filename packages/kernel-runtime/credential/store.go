package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

var (
	// ErrInvalidCiphertext is returned when ciphertext cannot be decrypted
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// Store provides encrypted credential storage
type Store struct {
	aead cipher.AEAD
}

// NewStore creates a new encrypted credential store from a 32-byte key
func NewStore(key []byte) (*Store, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return &Store{aead: aead}, nil
}

// Encrypt encrypts credential material and returns a base64-encoded string
func (s *Store) Encrypt(material *contracts.CredentialMaterial) (string, error) {
	if material == nil {
		return "", errors.New("credential material is nil")
	}

	plaintext := encodeMaterial(material)
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	sealed := s.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded credential blob
func (s *Store) Decrypt(encoded string) (*contracts.CredentialMaterial, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode credential: %w", err)
	}

	nonceSize := s.aead.NonceSize()
	if len(raw) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	return decodeMaterial(plaintext)
}

// encodeMaterial serializes credential material for encryption
func encodeMaterial(m *contracts.CredentialMaterial) []byte {
	buf := make([]byte, 0, len(m.Data)*64)
	for k, v := range m.Data {
		buf = append(buf, []byte(k)...)
		buf = append(buf, 0)
		buf = append(buf, []byte(v)...)
		buf = append(buf, 0)
	}
	return buf
}

// decodeMaterial deserializes decrypted credential material
func decodeMaterial(data []byte) (*contracts.CredentialMaterial, error) {
	result := &contracts.CredentialMaterial{Data: map[string]string{}}
	if len(data) == 0 {
		return result, nil
	}

	parts := splitNullTerminated(data)
	if len(parts)%2 != 0 {
		return nil, ErrInvalidCiphertext
	}
	for i := 0; i < len(parts); i += 2 {
		result.Data[string(parts[i])] = string(parts[i+1])
	}
	return result, nil
}

func splitNullTerminated(data []byte) [][]byte {
	var parts [][]byte
	start := 0
	for i, b := range data {
		if b == 0 {
			parts = append(parts, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		parts = append(parts, data[start:])
	}
	return parts
}
