package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/settings"
	"overline-connect/internal/whatsapp"
)

func handleGetSettings(repo settings.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		s, err := repo.Get(c.Request.Context(), company.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"antilink": s.AntilinkEnabled,
			"anticall": s.AnticallEnabled,
		})
	}
}

// handleUpdateSettings modifie les paramètres antilien/anti-appel de
// l'entreprise. Le changement d'anti-appel s'applique immédiatement au
// Provider WhatsApp de l'entreprise s'il existe déjà (pas besoin de
// reconnecter la session).
func handleUpdateSettings(repo settings.Repository, manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		current, err := repo.Get(c.Request.Context(), company.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
			return
		}

		var req struct {
			Antilink *bool `json:"antilink"`
			Anticall *bool `json:"anticall"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.Antilink != nil {
			current.AntilinkEnabled = *req.Antilink
		}
		if req.Anticall != nil {
			current.AnticallEnabled = *req.Anticall
		}
		current.CompanyID = company.ID

		if err := repo.Save(c.Request.Context(), current); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save settings"})
			return
		}

		if provider, exists := manager.Get(company.ID); exists {
			provider.SetAutoRejectCalls(current.AnticallEnabled)
		}

		c.JSON(http.StatusOK, gin.H{
			"antilink": current.AntilinkEnabled,
			"anticall": current.AnticallEnabled,
		})
	}
}
