// Package config centralise la configuration de l'application, chargée
// depuis les variables d'environnement (avec valeurs par défaut pour le dev
// local).
package config

import "os"

// Config regroupe tous les paramètres de configuration de l'application.
type Config struct {
	// Port d'écoute du serveur HTTP.
	Port string

	// WhatsAppSessionsDir est le dossier où sont stockés les fichiers de
	// session whatsmeow, un par entreprise (nommé <company_id>.db).
	WhatsAppSessionsDir string

	// AuthDBPath est le chemin du fichier SQLite contenant les entreprises
	// et clés API (internal/auth), utilisé par l'implémentation SQLite par
	// défaut.
	AuthDBPath string

	// AuthDBURL est l'URL de connexion PostgreSQL (ex:
	// postgres://user:pass@host:5432/db), utilisée seulement si tu bascules
	// vers internal/auth/postgres (voir son commentaire de package).
	// Vide par défaut car SQLite reste le backend actif.
	AuthDBURL string

	// CRMDBPath est le chemin du fichier SQLite contenant les conversations
	// et messages (internal/crm).
	CRMDBPath string

	// SettingsDBPath est le chemin du fichier SQLite contenant les
	// paramètres de modération par entreprise (internal/settings).
	SettingsDBPath string
}

// Load lit la configuration depuis les variables d'environnement.
func Load() Config {
	return Config{
		Port:                getEnv("PORT", "8080"),
		WhatsAppSessionsDir: getEnv("WHATSAPP_SESSIONS_DIR", "secret/whatsapp"),
		AuthDBPath:          getEnv("AUTH_DB_PATH", "secret/auth.db"),
		AuthDBURL:           getEnv("AUTH_DB_URL", ""),
		CRMDBPath:           getEnv("CRM_DB_PATH", "secret/crm.db"),
		SettingsDBPath:      getEnv("SETTINGS_DB_PATH", "secret/settings.db"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
