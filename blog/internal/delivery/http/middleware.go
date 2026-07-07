package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
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

// AuthMiddleware intercepts HTTP requests to parse, validate, and check whitelisted active sessions from Redis.
func AuthMiddleware(secretKey string, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
			c.Abort()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid token format."})
			c.Abort()
			return
		}

		tokenStr := tokenParts[1]
		claims, err := VerifyToken(tokenStr, secretKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Given token not valid for any token type."})
			c.Abort()
			return
		}

		if claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Token is not an access token."})
			c.Abort()
			return
		}

		// Enforce Redis active session limit checks to verify session has not been superseded
		if rdb != nil {
			redisKey := fmt.Sprintf("active_sessions:%s:%d", claims.SubKind, claims.SubID)
			val, err := rdb.Get(c.Request.Context(), redisKey).Result()
			if err == nil && val != "" {
				var activeSessions []string
				if err := json.Unmarshal([]byte(val), &activeSessions); err == nil {
					found := false
					for _, jti := range activeSessions {
						if jti == claims.RefreshJTI {
							found = true
							break
						}
					}
					if !found {
						c.JSON(http.StatusUnauthorized, gin.H{"detail": "This session has been terminated because you logged in on another device."})
						c.Abort()
						return
					}
				}
			}
		}

		// Set context values
		c.Set("userID", claims.SubID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// OptionalAuthMiddleware parses the JWT if present, but lets the request proceed without error if missing.
// This is essential to feed lists or article details where guests receive generic results and users receive personalized results.
func OptionalAuthMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && strings.ToLower(tokenParts[0]) == "bearer" {
				tokenStr := tokenParts[1]
				if claims, err := VerifyToken(tokenStr, secretKey); err == nil && claims.TokenType == "access" {
					c.Set("userID", claims.SubID)
					c.Set("role", claims.Role)
				}
			}
		}
		c.Next()
	}
}

// RequireAdmin blocks access if the injected role key doesn't equal "admin".
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    "forbidden",
				"message": "Only admins are allowed to perform this action.",
				"data":    nil,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

