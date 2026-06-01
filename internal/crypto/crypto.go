package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	MasterKeyBytes = 32
	nonceBytes     = 12
)

var ErrInvalidMasterKey = errors.New("crypto: invalid master key file")

type Cipher struct {
	aead cipher.AEAD
}

func LoadOrCreateCipher(path string) (*Cipher, error) {
	key, err := loadOrCreateMasterKey(path)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes init: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm init: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) EncryptString(plaintext string) (string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) DecryptString(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}
	if len(raw) < nonceBytes {
		return "", ErrInvalidMasterKey
	}
	nonce, ct := raw[:nonceBytes], raw[nonceBytes:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: open: %w", err)
	}
	return string(pt), nil
}

func loadOrCreateMasterKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		if len(data) != MasterKeyBytes {
			return nil, fmt.Errorf("%w: expected %d bytes at %s, got %d", ErrInvalidMasterKey, MasterKeyBytes, path, len(data))
		}
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("crypto: read master key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("crypto: mkdir master key dir: %w", err)
	}
	key := make([]byte, MasterKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: generate master key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("crypto: write master key: %w", err)
	}
	return key, nil
}
