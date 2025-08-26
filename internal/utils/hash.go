package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// CalculateSHA256 calculates the SHA256 hash of the given reader
func CalculateSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("failed to calculate SHA256: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 verifies that the data from reader matches the expected SHA256 hash
func VerifySHA256(r io.Reader, expectedHash string) error {
	calculatedHash, err := CalculateSHA256(r)
	if err != nil {
		return err
	}
	if calculatedHash != expectedHash {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, calculatedHash)
	}
	return nil
}