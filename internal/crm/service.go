package crm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"overline-connect/internal/whatsapp"
)

// Service contient la logique métier du CRM/Inbox. Il ne dépend que de
// l'interface Repository.
type Service struct {
	repo Repository
}

// NewService crée un Service à partir d'un Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// HandleMessage enregistre un message WhatsApp (reçu ou envoyé) pour une
// entreprise. Prévu pour être branché directement sur
// whatsapp.Provider.SetMessageHandler.
func (s *Service) HandleMessage(ctx context.Context, companyID string, m whatsapp.IncomingMessage) error {
	msg := Message{
		ID:        uuid.New().String(),
		CompanyID: companyID,
		ChatJID:   m.ChatJID,
		SenderJID: m.SenderJID,
		Text:      m.Text,
		FromMe:    m.FromMe,
		Timestamp: m.Timestamp,
	}
	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return fmt.Errorf("échec d'enregistrement du message: %w", err)
	}
	return nil
}

// ListConversations retourne les conversations d'une entreprise.
func (s *Service) ListConversations(ctx context.Context, companyID string) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, companyID)
}

// ListMessages retourne les messages d'une conversation d'une entreprise.
func (s *Service) ListMessages(ctx context.Context, companyID, chatJID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListMessages(ctx, companyID, chatJID, limit)
}
