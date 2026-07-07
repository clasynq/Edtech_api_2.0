package http

import (
	"net/http"
	"strings"

	"clasynq/api/admin/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware intercepts HTTP requests to parse and validate incoming Bearer access tokens.
// Upon successful validation, it injects "userID" and "role" values into Gin context.
func AuthMiddleware(secretKey string) gin.HandlerFunc {
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

		// Ensure we are working with access tokens rather than refresh tokens.
		if claims.TokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{"detail": "Token is not an access token."})
			c.Abort()
			return
		}

		// Inject properties to identify requester details in subsequent route controllers.
		c.Set("userID", claims.SubID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireAdmin verifies that the authenticated requester's injected role matches "admin".
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

