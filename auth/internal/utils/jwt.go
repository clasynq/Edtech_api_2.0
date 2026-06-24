package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

type DjangoClaims struct {
	SubKind    string `json:"sub_kind"`
	SubID      int64  `json:"sub_id"`
	UserID     int64  `json:"user_id"` // For backward compatibility/blueprint support
	Role       string `json:"role"`
	TokenType  string `json:"token_type"`
	RefreshJTI string `json:"refresh_jti,omitempty"`
	jwt.RegisteredClaims
}

// GenerateRandomJTI creates a random JTI identifier
func GenerateRandomJTI() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateTokenPair creates simple_jwt-compatible access & refresh token pair and registers sessions in Redis
func GenerateTokenPair(
	ctx context.Context,
	rdb *redis.Client,
	subKind string,
	subID int64,
	role string,
	oldJti string,
	secretKey string,
	accessTTLSec, refreshTTLSec int64,
) (map[string]string, error) {
	refreshJti := GenerateRandomJTI()
	accessJti := GenerateRandomJTI()

	now := time.Now()

	// 1. Generate Refresh Token
	refreshClaims := &DjangoClaims{
		SubKind:   subKind,
		SubID:     subID,
		UserID:    subID,
		Role:      role,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        refreshJti,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(refreshTTLSec) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenStr, err := refreshTokenObj.SignedString([]byte(secretKey))
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// 2. Generate Access Token
	accessClaims := &DjangoClaims{
		SubKind:    subKind,
		SubID:      subID,
		UserID:     subID,
		Role:       role,
		TokenType:  "access",
		RefreshJTI: refreshJti,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessJti,
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(accessTTLSec) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessTokenObj.SignedString([]byte(secretKey))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 3. Enforce device session limits in Redis if Redis is configured
	if rdb != nil {
		redisKey := fmt.Sprintf("active_sessions:%s:%d", subKind, subID)
		activeSessions := []string{}

		// Fetch existing sessions
		val, err := rdb.Get(ctx, redisKey).Result()
		if err == nil && val != "" {
			_ = json.Unmarshal([]byte(val), &activeSessions)
		}

		if oldJti != "" {
			// Refresh flow: replace the old session JTI with the new one
			found := false
			for i, jti := range activeSessions {
				if jti == oldJti {
					activeSessions[i] = refreshJti
					found = true
					break
				}
			}
			if !found {
				return nil, errors.New("Session has been terminated from this device.")
			}
		} else {
			// New login flow: evict oldest session if limit of 2 is exceeded
			if len(activeSessions) >= 2 {
				activeSessions = activeSessions[1:]
			}
			activeSessions = append(activeSessions, refreshJti)
		}

		sessionData, err := json.Marshal(activeSessions)
		if err == nil {
			// Keep sessions alive for refresh token lifetime
			_ = rdb.Set(ctx, redisKey, sessionData, time.Duration(refreshTTLSec)*time.Second).Err()
		}
	}

	return map[string]string{
		"access_token":  accessTokenStr,
		"refresh_token": refreshTokenStr,
	}, nil
}

// VerifyToken decodes and validates a token string
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
