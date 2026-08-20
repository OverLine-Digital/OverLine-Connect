package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/auth"
)

// companyContextKey est la clé sous laquelle l'entreprise authentifiée est
// stockée dans le contexte Gin par apiKeyMiddleware.
const companyContextKey = "company"

// apiKeyMiddleware authentifie chaque requête via une clé API
// (header "Authorization: Bearer olc_..."), et place l'entreprise
// correspondante dans le contexte pour les handlers suivants.
func apiKeyMiddleware(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		key := strings.TrimPrefix(header, "Bearer ")
		if key == "" || key == header {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			c.Abort()
			return
		}

		company, err := authService.VerifyAPIKey(c.Request.Context(), key)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			c.Abort()
			return
		}

		c.Set(companyContextKey, company)
		c.Next()
	}
}
