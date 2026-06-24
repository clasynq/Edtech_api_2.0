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
		claims, err := utils.VerifyToken(tokenStr, secretKey)
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

		// Enforce Redis active session limit checks
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
