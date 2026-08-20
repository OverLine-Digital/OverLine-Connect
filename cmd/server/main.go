// Command server démarre OverLine Connect : expose l'API HTTP ; chaque
// entreprise connecte sa propre session WhatsApp via POST /whatsapp/connect
// (voir README.md).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"overline-connect/internal/auth"
	authsqlite "overline-connect/internal/auth/sqlite"
	"overline-connect/internal/config"
	"overline-connect/internal/crm"
	crmsqlite "overline-connect/internal/crm/sqlite"
	"overline-connect/internal/httpapi"
	"overline-connect/internal/moderation"
	settingssqlite "overline-connect/internal/settings/sqlite"
	"overline-connect/internal/whatsapp"
	"overline-connect/internal/whatsapp/meow"
)

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.WhatsAppSessionsDir, 0o700); err != nil {
		log.Fatal("Échec de création du dossier de sessions whatsapp:", err)
	}

	// Service CRM : reçoit chaque message (via le handler branché dans la
	// Factory ci-dessous) et le rend consultable via /conversations.
	crmRepo, err := crmsqlite.New(cfg.CRMDBPath)
	if err != nil {
		log.Fatal("Échec de création du store crm:", err)
	}
	crmService := crm.NewService(crmRepo)

	// Store des paramètres de modération par entreprise (antilien,
	// anti-appel — voir internal/settings).
	settingsRepo, err := settingssqlite.New(cfg.SettingsDBPath)
	if err != nil {
		log.Fatal("Échec de création du store settings:", err)
	}

	// Manager + Factory : chaque entreprise obtient son propre Provider
	// whatsmeow, avec un fichier de session dédié et un handler de messages
	// qui alimente le CRM avec le bon company_id. Pour changer
	// d'implémentation (ex: Meta Cloud API en production), c'est la Factory
	// qu'il faut remplacer — le reste de l'application ne connaît que
	// l'interface whatsapp.Provider.
	factory := func(companyID string) (whatsapp.Provider, error) {
		dbPath := filepath.Join(cfg.WhatsAppSessionsDir, companyID+".db")
		provider, err := meow.New(meow.Config{SessionDBPath: dbPath})
		if err != nil {
			return nil, err
		}

		// Applique le réglage anti-appel déjà enregistré pour cette
		// entreprise (si elle en a déjà défini un).
		s, err := settingsRepo.Get(context.Background(), companyID)
		if err != nil {
			log.Println("Échec de lecture des paramètres, anti-appel désactivé par défaut:", err)
		} else {
			provider.SetAutoRejectCalls(s.AnticallEnabled)
		}

		provider.SetMessageHandler(func(msg whatsapp.IncomingMessage) {
			if err := crmService.HandleMessage(context.Background(), companyID, msg); err != nil {
				log.Println("Échec d'enregistrement du message CRM:", err)
			}

			// Antilien : dans un groupe, avertit si un message contient un
			// lien et que l'entreprise a activé cette protection.
			if !msg.FromMe && moderation.IsGroupChat(msg.ChatJID) && moderation.ContainsLink(msg.Text) {
				s, err := settingsRepo.Get(context.Background(), companyID)
				if err == nil && s.AntilinkEnabled {
					warning := "⚠️ Les liens ne sont pas autorisés dans ce groupe."
					if err := provider.SendText(context.Background(), msg.ChatJID, warning); err != nil {
						log.Println("Échec d'envoi de l'avertissement antilien:", err)
					}
				}
			}
		})

		return provider, nil
	}
	manager := whatsapp.NewManager(factory)

	// Construction du service d'authentification. Comme pour WhatsApp,
	// changer de moteur de stockage (ex: PostgreSQL) se fait ici uniquement
	// — le reste de l'application ne connaît que l'interface auth.Repository.
	authRepo, err := authsqlite.New(cfg.AuthDBPath)
	if err != nil {
		log.Fatal("Échec de création du store auth:", err)
	}
	authService := auth.NewService(authRepo)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	router := httpapi.NewRouter(manager, authService, crmService, settingsRepo)

	go func() {
		log.Println("Serveur démarré sur le port", cfg.Port)
		if err := router.Run(":" + cfg.Port); err != nil {
			log.Fatal("Échec du serveur HTTP:", err)
		}
	}()

	<-ctx.Done()
	log.Println("Arrêt en cours...")
}
