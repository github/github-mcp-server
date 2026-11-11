package requeststate

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const keySize = 32

// Sealer protects request state with AES-256-GCM.
type Sealer struct {
	aead cipher.AEAD
}

// NewRandom constructs a sealer with a process-local random key.
func NewRandom() (*Sealer, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}
	return newFromKey(key)
}

// New constructs a sealer from a standard Base64-encoded 32-byte key.
func New(encodedKey string) (*Sealer, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("decoded key must be %d bytes, got %d", keySize, len(key))
	}
	return newFromKey(key)
}

func newFromKey(key []byte) (*Sealer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts and authenticates plaintext into a URL-safe opaque token.
func (s *Sealer) Seal(_ context.Context, plaintext []byte) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	sealed := s.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open verifies and decrypts a token produced by Seal.
func (s *Sealer) Open(token string) ([]byte, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	nonceSize := s.aead.NonceSize()
	if len(sealed) < nonceSize {
		return nil, errors.New("token is too short")
	}
	plaintext, err := s.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("opening token: %w", err)
	}
	return plaintext, nil
}
