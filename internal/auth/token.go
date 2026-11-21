package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// NewToken returns a URL-safe random token and its SHA-256 hex hash for storage.
func NewToken(nbytes int) (plain, hash string, err error) {
	if nbytes < 16 {
		nbytes = 32
	}
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(sum[:]), nil
}

// HashToken hashes a presented token the same way NewToken stores it.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// NewSetupToken creates a human-pasteable setup token (wbr-setup-…).
func NewSetupToken() (plain, hash string, err error) {
	raw, _, err := NewToken(24)
	if err != nil {
		return "", "", err
	}
	plain = fmt.Sprintf("wbr-setup-%s", raw)
	return plain, HashToken(plain), nil
}
