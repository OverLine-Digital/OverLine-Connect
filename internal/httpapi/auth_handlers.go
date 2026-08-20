package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/auth"
)

func handleSignUp(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.Name == "" || req.Email == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name, email and password are required"})
			return
		}
		if len(req.Password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
			return
		}

		company, apiKey, err := authService.SignUp(c.Request.Context(), req.Name, req.Email, req.Password)
		if err != nil {
			if errors.Is(err, auth.ErrDuplicateEmail) {
				c.JSON(http.StatusConflict, gin.H{"error": "a company with this email already exists"})
				return
			}
			log.Println("signup failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create company"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"company_id": company.ID,
			"name":       company.Name,
			"api_key":    apiKey,
			"warning":    "store this API key now — it will not be shown again",
		})
	}
}
