package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// GenerateSalt creates a cryptographically secure random alphanumeric salt string of the specified length.
func GenerateSalt(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// VerifyDjangoPassword verifies a plaintext password against a Django-compatible PBKDF2 hash.
// The Django hash is structured as: pbkdf2_sha256$<iterations>$<salt>$<hash_base64>
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

	// Compute PBKDF2 key bytes using SHA256 hashing algorithm
	dk := pbkdf2.Key([]byte(plainPassword), []byte(salt), iterations, 32, sha256.New)
	calculatedHashBase64 := base64.StdEncoding.EncodeToString(dk)

	// Use ConstantTimeCompare to prevent timing attacks.
	if subtle.ConstantTimeCompare([]byte(hashBase64), []byte(calculatedHashBase64)) == 1 {
		return true, nil
	}
	return false, nil
}

// EncodeDjangoPassword hashes a password and formats it to match Django's pbkdf2_sha256 hashing format.
func EncodeDjangoPassword(plainPassword, salt string, iterations int) string {
	dk := pbkdf2.Key([]byte(plainPassword), []byte(salt), iterations, 32, sha256.New)
	hashBase64 := base64.StdEncoding.EncodeToString(dk)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iterations, salt, hashBase64)
}

