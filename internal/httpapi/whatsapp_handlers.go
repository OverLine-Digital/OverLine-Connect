package httpapi

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/auth"
	"overline-connect/internal/whatsapp"
)

// providerForCompany récupère (en le créant si besoin) le Provider WhatsApp
// de l'entreprise authentifiée sur la requête en cours.
func providerForCompany(c *gin.Context, manager *whatsapp.Manager) (whatsapp.Provider, bool) {
	company, ok := companyFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return nil, false
	}

	provider, err := manager.GetOrCreate(company.ID)
	if err != nil {
		// Log détaillé côté serveur (visible dans les logs Railway) — le
		// message renvoyé au client reste générique pour ne pas exposer de
		// détails internes.
		log.Println("providerForCompany: échec GetOrCreate pour l'entreprise", company.ID, ":", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to access whatsapp provider"})
		return nil, false
	}
	return provider, true
}

func companyFromContext(c *gin.Context) (auth.Company, bool) {
	v, exists := c.Get(companyContextKey)
	if !exists {
		return auth.Company{}, false
	}
	company, ok := v.(auth.Company)
	return company, ok
}

// handleWhatsAppConnect démarre (ou relance) la connexion WhatsApp de
// l'entreprise. Non-bloquant : si une session existe déjà, elle se
// reconnecte ; sinon un QR code devient disponible via GET /whatsapp/status.
func handleWhatsAppConnect(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, ok := providerForCompany(c, manager)
		if !ok {
			return
		}

		if err := provider.Connect(c.Request.Context()); err != nil {
			log.Println("handleWhatsAppConnect: échec Connect:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start whatsapp connection"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": provider.Status()})
	}
}

// handleWhatsAppStatus retourne l'état de connexion et, le cas échéant, le
// QR code à scanner.
func handleWhatsAppStatus(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		company, ok := companyFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
			return
		}

		provider, exists := manager.Get(company.ID)
		if !exists {
			c.JSON(http.StatusOK, gin.H{"status": whatsapp.StatusDisconnected})
			return
		}

		resp := gin.H{"status": provider.Status()}
		if code, ok := provider.QRCode(); ok {
			resp["qr_code"] = code
		}
		c.JSON(http.StatusOK, resp)
	}
}

// handlePairPhone démarre une connexion par code d'appairage (alternative
// au QR code, utile sur une connexion instable). Attend un corps JSON
// {"phone": "243900000000"}.
func handlePairPhone(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, ok := providerForCompany(c, manager)
		if !ok {
			return
		}

		var req struct {
			Phone string `json:"phone"`
		}
		if err := c.BindJSON(&req); err != nil || req.Phone == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "phone number required"})
			return
		}

		code, err := provider.RequestPairingCode(c.Request.Context(), req.Phone)
		if err != nil {
			log.Println("handlePairPhone: échec:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to request pairing code"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pairing_code": code})
	}
}
