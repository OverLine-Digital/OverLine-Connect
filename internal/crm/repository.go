package crm

import "context"

// Repository est le contrat de stockage pour les conversations et messages.
// L'implémentation actuelle est en SQLite (internal/crm/sqlite) ; une
// implémentation PostgreSQL pourra la remplacer sans toucher à Service ni
// aux appelants HTTP.
type Repository interface {
	// SaveMessage enregistre un message et met à jour (ou crée) la
	// conversation correspondante.
	SaveMessage(ctx context.Context, msg Message) error

	// ListConversations retourne les conversations d'une entreprise,
	// triées par message le plus récent d'abord.
	ListConversations(ctx context.Context, companyID string) ([]Conversation, error)

	// ListMessages retourne les messages d'une conversation, du plus ancien
	// au plus récent, limités à limit résultats (les plus récents).
	ListMessages(ctx context.Context, companyID, chatJID string, limit int) ([]Message, error)
}
