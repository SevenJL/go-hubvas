package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/domain/identity"
)

// TokenValidator is the minimal JWT validation interface.
type TokenValidator interface {
	ValidateAccessToken(tokenString string) (identity.UserID, error)
}

// AuthMiddleware validates JWT tokens and injects the user ID into the context.
func AuthMiddleware(tokenSvc TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "missing or invalid authorization header",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := tokenSvc.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "invalid or expired token",
			})
			return
		}

		// Inject into Gin context for downstream handlers.
		c.Set("userID", userID)
		c.Next()
	}
}

// OptionalAuthMiddleware authenticates a request when a Bearer token is present.
// Requests without credentials remain anonymous, while invalid credentials are rejected.
func OptionalAuthMiddleware(tokenSvc TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "invalid authorization header",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := tokenSvc.ValidateAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    http.StatusUnauthorized,
				"message": "invalid or expired token",
			})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// GetUserID extracts the authenticated user ID from the Gin context.
// Returns 0 if not set (should only be called behind AuthMiddleware).
func GetUserID(c *gin.Context) identity.UserID {
	val, exists := c.Get("userID")
	if !exists {
		return 0
	}
	return val.(identity.UserID)
}
