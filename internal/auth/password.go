package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword returns a bcrypt hash of password.
func HashPassword(password string) (string, error) {
	if len(password) < 10 {
		return "", fmt.Errorf("password must be at least 10 characters")
	}
	if len(password) > 200 {
		return "", fmt.Errorf("password too long")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword compares password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
