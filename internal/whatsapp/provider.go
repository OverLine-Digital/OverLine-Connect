// Package whatsapp définit le contrat que toute implémentation WhatsApp doit
// respecter. Le reste de l'application (API HTTP, futur CRM, futur module IA)
// ne dépend que de cette interface, jamais directement de whatsmeow ou de la
// Cloud API Meta. Ça permet de commencer avec whatsmeow (dev/test) et de
// basculer plus tard vers l'API officielle Meta sans toucher au reste du code.
package whatsapp

import (
	"context"
	"time"
)

// IncomingMessage représente un message WhatsApp (reçu ou envoyé) transmis
// au reste de l'application via le handler configuré par SetMessageHandler.
type IncomingMessage struct {
	// ChatJID identifie la conversation (le contact, pour une discussion
	// individuelle).
	ChatJID string
	// SenderJID identifie l'auteur du message.
	SenderJID string
	Text      string
	// FromMe indique si le message a été envoyé par l'entreprise elle-même
	// (via l'API ou depuis le téléphone) plutôt que reçu d'un client.
	FromMe    bool
	Timestamp time.Time
}

// MessageHandler est appelé pour chaque message (reçu ou envoyé) sur la
// session. Utilisé par internal/crm pour construire l'historique des
// conversations.
type MessageHandler func(IncomingMessage)


// Status représente l'état de connexion d'un Provider.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusAwaitingScan Status = "awaiting_scan"
	StatusConnected    Status = "connected"
)

// Provider est implémenté par chaque backend WhatsApp (whatsmeow aujourd'hui,
// Meta Cloud API plus tard).
//
// Connect est NON-BLOQUANT : si une session existe déjà, elle se reconnecte
// rapidement ; sinon elle démarre le processus de pairage en arrière-plan et
// retourne immédiatement. Appeler Status() et QRCode() pour suivre la
// progression — c'est ce qui permet d'exposer le QR code via une route HTTP
// plutôt que de bloquer une requête en attendant un scan.
type Provider interface {
	Connect(ctx context.Context) error
	Disconnect()

	// Status retourne l'état de connexion actuel.
	Status() Status

	// QRCode retourne le dernier QR code généré (à afficher au client pour
	// scan) et un booléen indiquant s'il est valide. Utile seulement quand
	// Status() == StatusAwaitingScan.
	QRCode() (code string, ok bool)

	SendText(ctx context.Context, to, text string) error
	SendImage(ctx context.Context, to string, data []byte, caption string) error
	SendDocument(ctx context.Context, to string, data []byte, filename, caption string) error

	// SetMessageHandler enregistre la fonction appelée pour chaque message
	// texte reçu ou envoyé sur cette session. Remplace tout handler
	// précédemment défini. Passer nil pour désactiver.
	SetMessageHandler(handler MessageHandler)

	// SetAutoRejectCalls active ou désactive le rejet automatique des
	// appels entrants sur cette session (fonctionnalité "anti-appel").
	SetAutoRejectCalls(enabled bool)

	// RequestPairingCode démarre une connexion par code d'appairage
	// (alternative au QR code) pour le numéro de téléphone donné (format
	// international, sans "+", ex: "243900000000"). Retourne un code à
	// saisir dans WhatsApp (Paramètres → Appareils liés → Lier avec un
	// numéro de téléphone).
	RequestPairingCode(ctx context.Context, phoneNumber string) (string, error)
}
