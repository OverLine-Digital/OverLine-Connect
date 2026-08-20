// Package httpapi expose l'API HTTP de l'application. Les handlers ne
// dépendent que de l'interface whatsapp.Provider — jamais d'une
// implémentation concrète — pour rester agnostiques du backend WhatsApp
// utilisé (whatsmeow aujourd'hui, Meta Cloud API plus tard).
package httpapi

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"overline-connect/internal/auth"
	"overline-connect/internal/crm"
	"overline-connect/internal/settings"
	"overline-connect/internal/webui"
	"overline-connect/internal/whatsapp"
)

// NewRouter construit le routeur Gin avec toutes les routes de l'API.
// Chaque entreprise a sa propre session WhatsApp, gérée par manager
// (internal/whatsapp.Manager) et identifiée via sa clé API.
func NewRouter(manager *whatsapp.Manager, authService *auth.Service, crmService *crm.Service, settingsRepo settings.Repository) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", webui.DashboardHTML)
	})

	r.POST("/auth/signup", handleSignUp(authService))

	protected := r.Group("/")
	protected.Use(apiKeyMiddleware(authService))

	protected.POST("/whatsapp/connect", handleWhatsAppConnect(manager))
	protected.GET("/whatsapp/status", handleWhatsAppStatus(manager))
	protected.POST("/whatsapp/pair-phone", handlePairPhone(manager))

	protected.POST("/sendtext", handleSendText(manager))
	protected.POST("/senddoc", handleSendDocument(manager))
	protected.POST("/sendimg", handleSendImage(manager))

	protected.GET("/conversations", handleListConversations(crmService))
	protected.GET("/conversations/:chatJID/messages", handleListMessages(crmService))

	// Paramètres de modération (antilien, anti-appel — inspirés de
	// Toxic-MD, adaptés au multi-tenant).
	protected.GET("/settings", handleGetSettings(settingsRepo))
	protected.POST("/settings", handleUpdateSettings(settingsRepo, manager))

	return r
}

func handleSendText(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, ok := providerForCompany(c, manager)
		if !ok {
			return
		}
		if provider.Status() != whatsapp.StatusConnected {
			c.JSON(http.StatusConflict, gin.H{"error": "whatsapp not connected — call POST /whatsapp/connect first"})
			return
		}

		var req struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		clientIP := c.ClientIP()
		log.Println("Requests received from", clientIP)

		if err := provider.SendText(c.Request.Context(), req.To, req.Text); err != nil {
			log.Println("Failed to send message to", req.To, ":", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send message"})
			return
		}

		log.Println("Success sending message to", req.To)
		c.JSON(http.StatusOK, gin.H{"message": "message sent successfully"})
	}
}

func handleSendDocument(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, ok := providerForCompany(c, manager)
		if !ok {
			return
		}
		if provider.Status() != whatsapp.StatusConnected {
			c.JSON(http.StatusConflict, gin.H{"error": "whatsapp not connected — call POST /whatsapp/connect first"})
			return
		}

		var req struct {
			To       string `json:"to"`
			Caption  string `json:"caption"`
			Filename string `json:"filename"`
			Document []byte `json:"document"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		clientIP := c.ClientIP()
		log.Println("Requests received from", clientIP)

		if err := provider.SendDocument(c.Request.Context(), req.To, req.Document, req.Filename, req.Caption); err != nil {
			log.Println("Failed to send document to", req.To, ":", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send document"})
			return
		}

		log.Println("Success sending document to", req.To)
		c.JSON(http.StatusOK, gin.H{"message": "document sent successfully"})
	}
}

func handleSendImage(manager *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, ok := providerForCompany(c, manager)
		if !ok {
			return
		}
		if provider.Status() != whatsapp.StatusConnected {
			c.JSON(http.StatusConflict, gin.H{"error": "whatsapp not connected — call POST /whatsapp/connect first"})
			return
		}

		var req struct {
			To      string `json:"to"`
			Caption string `json:"caption"`
			Image   []byte `json:"image"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		clientIP := c.ClientIP()
		log.Println("Requests received from", clientIP)

		if err := provider.SendImage(c.Request.Context(), req.To, req.Image, req.Caption); err != nil {
			log.Println("Failed to send image to", req.To, ":", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send image"})
			return
		}

		log.Println("Success sending image to", req.To)
		c.JSON(http.StatusOK, gin.H{"message": "image sent successfully"})
	}
}
