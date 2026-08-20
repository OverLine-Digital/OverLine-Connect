package meow

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"

	"overline-connect/internal/whatsapp"
)

// RequestPairingCode démarre la connexion sans QR code, via un code
// d'appairage à saisir manuellement dans WhatsApp (Paramètres → Appareils
// liés → Lier avec un numéro de téléphone).
//
// API vérifiée via la documentation officielle whatsmeow
// (pkg.go.dev/go.mau.fi/whatsmeow) :
// PairPhone(ctx, phone, showPushNotification, clientType, clientDisplayName) (string, error).
func (p *Provider) RequestPairingCode(ctx context.Context, phoneNumber string) (string, error) {
	if p.client.Store.ID != nil {
		return "", fmt.Errorf("une session existe déjà pour cet appareil")
	}

	if err := p.client.Connect(); err != nil {
		return "", fmt.Errorf("échec de démarrage de la connexion: %w", err)
	}

	p.setStatus(whatsapp.StatusAwaitingScan)

	code, err := p.client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "OverLine Connect")
	if err != nil {
		return "", fmt.Errorf("échec de génération du code d'appairage: %w", err)
	}

	return code, nil
}
