package utils

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type DjangoClaims struct {
	SubKind    string `json:"sub_kind"`
	SubID      int64  `json:"sub_id"`
	UserID     int64  `json:"user_id"` // For backward compatibility
	Role       string `json:"role"`
	TokenType  string `json:"token_type"`
	RefreshJTI string `json:"refresh_jti,omitempty"`
	jwt.RegisteredClaims
}

func VerifyToken(tokenStr string, secretKey string) (*DjangoClaims, error) {
	claims := &DjangoClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
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
