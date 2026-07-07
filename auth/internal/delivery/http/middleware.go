package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"clasynq/api/auth/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// AuthMiddleware intercepts HTTP requests to authenticate users via Bearer JWT.
// It verifies token validity, checks for access token types, enforces device session limits
// against Redis active sessions, and stores userID/role in the Gin context.
func AuthMiddleware(secretKey string, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Retrieve the Authorization header value.
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Authentication credentials were not provided."})
			c.Abort()
			return
		}

		// 2. Validate token format (must be "Bearer <Token>").
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Invalid token format."})
			c.Abort()
			return
		}

		// 3. Verify JWT signature, expiration times, and claims.
		tokenStr := tokenParts[1]
		claims, err := utils.VerifyToken(tokenStr, secretKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Given token not valid for any token type."})
			c.Abort()
			return
		}

		// 4. Ensure it's a dedicated access token (refresh tokens cannot be used to call endpoints).
		if claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Token is not an access token."})
			c.Abort()
			return
		}

		// 5. Enforce active device session limiting using Redis.
		// If another login has occurred that deleted this token's JTI from the session whitelist, reject request.
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

		// 6. Set identity variables inside the request Context for downstream handlers to access.
		c.Set("userID", claims.SubID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

