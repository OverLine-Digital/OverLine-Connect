// Package auth gère l'authentification des entreprises : inscription,
// connexion, et clés API. Ne dépend d'aucun moteur de base de données
// précis — voir Repository. L'implémentation actuelle (sqlite/) est
// temporaire ; une implémentation PostgreSQL pourra la remplacer sans
// changer ce fichier ni les appelants.
package auth

import "time"

// Company représente une entreprise cliente d'OverLine Connect.
type Company struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// APIKey représente une clé API appartenant à une entreprise. Seul le hash
// est stocké ; la valeur en clair n'est retournée qu'une fois, à la création.
type APIKey struct {
	ID        string
	CompanyID string
	KeyHash   string
	CreatedAt time.Time
}
