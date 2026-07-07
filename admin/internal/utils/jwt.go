package utils

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// DjangoClaims holds JWT payload parameters. Designed specifically to match the structure
// of Django's Simple JWT library, facilitating authentication sharing across microservices.
type DjangoClaims struct {
	SubKind    string `json:"sub_kind"`             // Subject kind (e.g. admin, student, teacher)
	SubID      int64  `json:"sub_id"`               // Subject primary key ID
	UserID     int64  `json:"user_id"`              // Legacy User table reference (for backward compatibility)
	Role       string `json:"role"`                 // Assigned access permissions role
	TokenType  string `json:"token_type"`           // "access" or "refresh"
	RefreshJTI string `json:"refresh_jti,omitempty"` // Unique identifier for whitelisting refresh tokens
	jwt.RegisteredClaims
}

// VerifyToken decodes, parses, and validates the signature of a signed JWT string against secretKey.
func VerifyToken(tokenStr string, secretKey string) (*DjangoClaims, error) {
	claims := &DjangoClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// Enforce standard HMAC-SHA256 signature verification method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

