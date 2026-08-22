package meow

import (
	"context"
	"log"

	"go.mau.fi/whatsmeow/types/events"

	"overline-connect/internal/whatsapp"
)

// handleHistorySync traite l'historique de conversations que WhatsApp
// envoie automatiquement juste après qu'un nouvel appareil soit lié
// (comme quand on ouvre WhatsApp Web pour la première fois et qu'on voit
// apparaître toutes ses anciennes conversations).
//
// ATTENTION : cette fonctionnalité touche une partie moins documentée de
// whatsmeow que le reste du projet. Le nom exact des champs (Conversations,
// Messages, etc.) n'a pas pu être vérifié par compilation ni contre la
// documentation officielle dans l'environnement où ce code a été écrit. Si
// la compilation échoue sur ce fichier précis, envoie-moi l'erreur exacte —
// c'est très probablement juste un nom de champ à ajuster, le reste de
// l'application n'est pas affecté.
func (p *Provider) handleHistorySync(evt interface{}) {
	syncEvt, ok := evt.(*events.HistorySync)
	if !ok {
		return
	}

	p.mu.Lock()
	handler := p.messageHandler
	p.mu.Unlock()
	if handler == nil {
		return
	}

	conversations := syncEvt.Data.GetConversations()
	log.Printf("whatsmeow: historique reçu — %d conversation(s)", len(conversations))

	count := 0
	for _, conv := range conversations {
		chatJID := conv.GetID()

		for _, histMsg := range conv.GetMessages() {
			webMsg := histMsg.GetMessage()
			if webMsg == nil {
				continue
			}

			msg := webMsg.GetMessage()
			text := ""
			if msg != nil {
				if msg.GetConversation() != "" {
					text = msg.GetConversation()
				} else if ext := msg.GetExtendedTextMessage(); ext != nil {
					text = ext.GetText()
				}
			}
			if text == "" {
				continue
			}

			key := webMsg.GetKey()
			senderJID := key.GetRemoteJID()
			if key.GetParticipant() != "" {
				senderJID = key.GetParticipant()
			}

			handler(whatsapp.IncomingMessage{
				ChatJID:   chatJID,
				SenderJID: senderJID,
				Text:      text,
				FromMe:    key.GetFromMe(),
				Timestamp: timestampFromUnix(webMsg.GetMessageTimestamp()),
			})
			count++
		}
	}

	log.Printf("whatsmeow: historique traité — %d message(s) importé(s) dans le CRM", count)
}

// requestFullHistorySync demande à WhatsApp de renvoyer un historique aussi
// complet que possible (plutôt que le résumé minimal envoyé par défaut).
// Appelée juste après une connexion réussie.
func (p *Provider) requestFullHistorySync(_ context.Context) {
	// NOTE : whatsmeow envoie l'historique automatiquement après le
	// pairage, sans action explicite requise de notre part dans les
	// versions récentes — cette fonction ne fait rien pour l'instant,
	// conservée si un réglage explicite s'avère nécessaire après test.
}
