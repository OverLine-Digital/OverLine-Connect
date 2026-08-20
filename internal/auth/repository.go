package auth

import (
	"context"
	"errors"
)

// Erreurs retournées par les implémentations de Repository.
var (
	ErrNotFound      = errors.New("introuvable")
	ErrDuplicateEmail = errors.New("une entreprise existe déjà avec cet email")
)

// Repository est le contrat de stockage pour les entreprises et clés API.
// L'implémentation actuelle est en SQLite (internal/auth/sqlite) ; une
// implémentation PostgreSQL pourra la remplacer sans toucher à Service ni
// aux appelants HTTP — c'est tout l'intérêt de passer par cette interface.
type Repository interface {
	CreateCompany(ctx context.Context, company Company) error
	GetCompanyByEmail(ctx context.Context, email string) (Company, error)
	GetCompanyByID(ctx context.Context, id string) (Company, error)

	CreateAPIKey(ctx context.Context, key APIKey) error
	// GetCompanyByAPIKeyHash retrouve l'entreprise propriétaire d'une clé
	// API à partir du hash de cette clé (jamais de la clé en clair).
	GetCompanyByAPIKeyHash(ctx context.Context, keyHash string) (Company, error)
}
