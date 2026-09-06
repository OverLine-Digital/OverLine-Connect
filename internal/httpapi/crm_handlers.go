package httpapi

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/crm"
)

func handleListConversations(crmService *crm.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		conversations, err := crmService.ListConversations(c.Request.Context(), company.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list conversations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"conversations": conversations})
	}
}

// handleDebugCount est une route de diagnostic TEMPORAIRE : compte
// directement le nombre de messages réellement enregistrés en base pour
// l'entreprise, sans passer par la logique de regroupement par conversation
// (utile pour vérifier si les données sont bien écrites, indépendamment
// d'un éventuel bug d'affichage de l'Inbox). À retirer une fois le
// problème confirmé résolu.
func handleDebugCount(crmService *crm.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		count, err := crmService.CountMessages(c.Request.Context(), company.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message_count": count, "company_id": company.ID})
	}
}

func handleListMessages(crmService *crm.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		chatJID, err := url.QueryUnescape(c.Param("chatJID"))
		if err != nil || chatJID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
			return
		}

		limit := 50
		if raw := c.Query("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				limit = parsed
			}
		}

		messages, err := crmService.ListMessages(c.Request.Context(), company.ID, chatJID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list messages"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}
