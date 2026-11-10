package auth

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP key for the given account email.
func GenerateTOTPSecret(issuer, account string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a 6-digit code against the secret (skew ±1 period).
func ValidateTOTP(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return ok
}

// MustValidateTOTP is like ValidateTOTP but returns an error for callers that prefer it.
func MustValidateTOTP(secret, code string) error {
	if !ValidateTOTP(secret, code) {
		return fmt.Errorf("invalid TOTP code")
	}
	return nil
}
