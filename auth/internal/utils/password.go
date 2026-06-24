package utils

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// VerifyDjangoPassword validates a plaintext password against a Django-style hash
func VerifyDjangoPassword(plainPassword, djangoHash string) (bool, error) {
	parts := strings.Split(djangoHash, "$")
	if len(parts) != 4 {
		return false, errors.New("invalid django hash format")
	}

	algorithm := parts[0]
	if algorithm != "pbkdf2_sha256" {
		return false, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, fmt.Errorf("invalid iterations: %w", err)
	}

	salt := parts[2]
	hashBase64 := parts[3]

	// Django hashes using standard PBKDF2 with SHA-256
	dk := pbkdf2.Key([]byte(plainPassword), []byte(salt), iterations, 32, sha256.New)
	calculatedHashBase64 := base64.StdEncoding.EncodeToString(dk)

	// Constant-time compare to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(hashBase64), []byte(calculatedHashBase64)) == 1 {
		return true, nil
	}
	return false, nil
}

// EncodeDjangoPassword creates a Django-compatible pbkdf2_sha256 hash string
func EncodeDjangoPassword(plainPassword, salt string, iterations int) string {
	dk := pbkdf2.Key([]byte(plainPassword), []byte(salt), iterations, 32, sha256.New)
	hashBase64 := base64.StdEncoding.EncodeToString(dk)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iterations, salt, hashBase64)
}
