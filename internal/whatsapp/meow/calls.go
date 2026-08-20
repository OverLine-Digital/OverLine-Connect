package meow

import (
	"context"
	"log"

	"go.mau.fi/whatsmeow/types/events"
)

// handleCallEvent traite les événements d'appel whatsmeow. Si le rejet
// automatique est activé (voir SetAutoRejectCalls), rejette l'appel
// entrant.
//
// API vérifiée via la documentation officielle whatsmeow
// (pkg.go.dev/go.mau.fi/whatsmeow) : RejectCall(ctx, callFrom, callID)
// error, et types.BasicCallMeta expose bien les champs CallCreator et
// CallID utilisés ici.
func (p *Provider) handleCallEvent(evt interface{}) {
	callEvt, ok := evt.(*events.CallOffer)
	if !ok {
		return
	}

	p.mu.Lock()
	shouldReject := p.autoRejectCalls
	p.mu.Unlock()
	if !shouldReject {
		return
	}

	if err := p.client.RejectCall(context.Background(), callEvt.CallCreator, callEvt.CallID); err != nil {
		log.Println("Échec de rejet automatique de l'appel:", err)
	}
}
