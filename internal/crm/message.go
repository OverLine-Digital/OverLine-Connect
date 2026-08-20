// Package crm gère l'historique des conversations WhatsApp par entreprise :
// réception des messages, stockage, et consultation (inbox). Ne dépend
// d'aucun moteur de base de données précis — voir Repository.
package crm

import "time"

// Message représente un message WhatsApp (reçu ou envoyé), rattaché à une
// entreprise et une conversation.
type Message struct {
	ID        string
	CompanyID string
	ChatJID   string
	SenderJID string
	Text      string
	FromMe    bool
	Timestamp time.Time
}

// Conversation représente le fil de discussion avec un contact, pour une
// entreprise donnée. Recalculée à partir du dernier message reçu/envoyé.
type Conversation struct {
	ChatJID         string
	CompanyID       string
	LastMessageText string
	LastMessageAt   time.Time
}
