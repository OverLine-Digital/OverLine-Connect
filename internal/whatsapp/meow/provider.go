// Package meow implémente whatsapp.Provider avec la librairie whatsmeow
// (client WhatsApp non-officiel, type "WhatsApp Web"). Adapté au
// développement et aux tests. Pour la production à plus grande échelle,
// prévoir une migration vers la Meta Cloud API officielle (autre
// implémentation de whatsapp.Provider, dans un package séparé).
package meow

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite"

	"overline-connect/internal/whatsapp"
)

func init() {
	// Se présente comme un téléphone Android auprès de WhatsApp.
	store.DeviceProps.PlatformType = waProto.DeviceProps_ANDROID_PHONE.Enum()
	store.DeviceProps.Os = proto.String("OverLine Connect")
}

// Config regroupe les paramètres nécessaires pour créer un Provider whatsmeow.
type Config struct {
	// SessionDBPath est le chemin du fichier SQLite où whatsmeow stocke la
	// session de l'appareil (identifiants de connexion WhatsApp). Un
	// fichier distinct par entreprise assure l'isolation des sessions
	// (voir internal/whatsapp.Manager).
	SessionDBPath string
}

// Provider implémente whatsapp.Provider via whatsmeow.
type Provider struct {
	client *whatsmeow.Client

	mu              sync.Mutex
	status          whatsapp.Status
	qrCode          string
	messageHandler  whatsapp.MessageHandler
	autoRejectCalls bool
}

// New crée un Provider whatsmeow à partir de la config donnée. Elle ouvre le
// store de session mais n'établit pas encore la connexion réseau : appeler
// Connect() pour ça.
func New(cfg Config) (*Provider, error) {
	ctx := context.Background()

	dbLog := waLog.Stdout("Database", "ERROR", true)
	container, err := sqlstore.New(ctx, "sqlite", "file:"+cfg.SessionDBPath+"?_foreign_keys=on&_pragma=busy_timeout(10000)", dbLog)
	if err != nil {
		return nil, fmt.Errorf("échec de création du store de session: %w", err)
	}

	// Migre le schéma de la base de session (crée les tables nécessaires).
	// Sans cet appel, une base de données neuve reste vide et
	// GetFirstDevice() échoue silencieusement — cause probable des échecs
	// de connexion observés en production.
	if err := container.Upgrade(ctx); err != nil {
		return nil, fmt.Errorf("échec de migration du store de session: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("échec de récupération de l'appareil stocké: %w", err)
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	p := &Provider{
		client: client,
		status: whatsapp.StatusDisconnected,
	}

	// Écoute les messages entrants pour les transmettre au handler
	// (utilisé par internal/crm pour construire l'historique des
	// conversations). Les messages envoyés via SendText/SendImage/
	// SendDocument sont notifiés séparément, directement dans ces méthodes.
	client.AddEventHandler(func(evt interface{}) {
		msgEvt, ok := evt.(*events.Message)
		if !ok {
			return
		}
		text := extractText(msgEvt)
		if text == "" {
			return
		}

		p.mu.Lock()
		handler := p.messageHandler
		p.mu.Unlock()
		if handler == nil {
			return
		}

		handler(whatsapp.IncomingMessage{
			ChatJID:   msgEvt.Info.Chat.String(),
			SenderJID: msgEvt.Info.Sender.String(),
			Text:      text,
			FromMe:    msgEvt.Info.IsFromMe,
			Timestamp: msgEvt.Info.Timestamp,
		})
	})

	// Écoute les appels entrants pour le rejet automatique (fonctionnalité
	// "anti-appel"). Logique isolée dans calls.go — voir ce fichier si la
	// compilation échoue sur ce point précis, le reste de New() n'est pas
	// affecté.
	client.AddEventHandler(p.handleCallEvent)

	// Détecte la réussite de l'appairage par code (voir pairing.go) : passe
	// le statut à "connecté" une fois le code saisi côté téléphone.
	client.AddEventHandler(func(evt interface{}) {
		if _, ok := evt.(*events.PairSuccess); ok {
			p.setStatus(whatsapp.StatusConnected)
		}
	})

	return p, nil
}

// extractText récupère le texte d'un message whatsmeow, qu'il s'agisse d'un
// message simple ou d'un message texte enrichi (réponse à un message,
// mise en forme, etc.). Retourne une chaîne vide pour les messages sans
// contenu texte (image seule, etc.) — non gérés par le CRM à ce stade.
func extractText(evt *events.Message) string {
	msg := evt.Message
	if msg == nil {
		return ""
	}
	if msg.GetConversation() != "" {
		return msg.GetConversation()
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}
	return ""
}

// SetMessageHandler enregistre la fonction appelée pour chaque message
// texte reçu ou envoyé sur cette session.
func (p *Provider) SetMessageHandler(handler whatsapp.MessageHandler) {
	p.mu.Lock()
	p.messageHandler = handler
	p.mu.Unlock()
}

func (p *Provider) notifyOutgoing(to, text string) {
	p.mu.Lock()
	handler := p.messageHandler
	p.mu.Unlock()
	if handler == nil {
		return
	}
	jid, err := toJID(to)
	if err != nil {
		return
	}
	handler(whatsapp.IncomingMessage{
		ChatJID:   jid.String(),
		SenderJID: jid.String(),
		Text:      text,
		FromMe:    true,
		Timestamp: time.Now(),
	})
}

// Connect établit la connexion WhatsApp. NON-BLOQUANT :
//   - si une session existe déjà, elle se reconnecte de façon synchrone
//     (rapide, pas d'attente de scan) ;
//   - sinon, elle démarre le pairage en arrière-plan et retourne
//     immédiatement — suivre la progression via Status() et QRCode().
func (p *Provider) Connect(ctx context.Context) error {
	if p.client.Store.ID != nil {
		if err := p.client.Connect(); err != nil {
			return fmt.Errorf("échec de connexion à whatsapp: %w", err)
		}
		p.setStatus(whatsapp.StatusConnected)
		return nil
	}

	qrChan, err := p.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("échec d'ouverture du canal qrcode: %w", err)
	}
	if err := p.client.Connect(); err != nil {
		return fmt.Errorf("échec de démarrage de la connexion: %w", err)
	}

	p.setStatus(whatsapp.StatusAwaitingScan)

	go func() {
		for evt := range qrChan {
			log.Println("whatsmeow QR event:", evt.Event)
			switch evt.Event {
			case "code":
				p.setQRCode(evt.Code)
			case "success":
				p.setStatus(whatsapp.StatusConnected)
			case "err-client-outdated", "timeout":
				p.setStatus(whatsapp.StatusDisconnected)
			}
		}
		log.Println("whatsmeow QR: canal fermé (fin de boucle)")
	}()

	return nil
}

// Disconnect ferme la connexion WhatsApp.
func (p *Provider) Disconnect() {
	p.client.Disconnect()
	p.setStatus(whatsapp.StatusDisconnected)
}

// Status retourne l'état de connexion actuel.
func (p *Provider) Status() whatsapp.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

// QRCode retourne le dernier QR code généré, valide seulement pendant
// StatusAwaitingScan.
func (p *Provider) QRCode() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status != whatsapp.StatusAwaitingScan || p.qrCode == "" {
		return "", false
	}
	return p.qrCode, true
}

func (p *Provider) setStatus(s whatsapp.Status) {
	p.mu.Lock()
	p.status = s
	if s != whatsapp.StatusAwaitingScan {
		p.qrCode = ""
	}
	p.mu.Unlock()
}

func (p *Provider) setQRCode(code string) {
	p.mu.Lock()
	p.qrCode = code
	p.mu.Unlock()
}

// SendText envoie un message texte simple.
func (p *Provider) SendText(ctx context.Context, to, text string) error {
	jid, err := toJID(to)
	if err != nil {
		return err
	}

	_, err = p.client.SendMessage(ctx, jid, &waProto.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return fmt.Errorf("échec d'envoi du message: %w", err)
	}
	p.notifyOutgoing(to, text)
	return nil
}

// SendImage envoie une image avec légende optionnelle.
func (p *Provider) SendImage(ctx context.Context, to string, data []byte, caption string) error {
	jid, err := toJID(to)
	if err != nil {
		return err
	}

	media, err := p.client.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("échec d'upload de l'image: %w", err)
	}

	_, err = p.client.SendMessage(ctx, jid, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Mimetype:      proto.String(http.DetectContentType(data)),
			StaticURL:     proto.String(media.URL),
			DirectPath:    proto.String(media.DirectPath),
			MediaKey:      media.MediaKey,
			FileEncSHA256: media.FileEncSHA256,
			FileSHA256:    media.FileSHA256,
			FileLength:    proto.Uint64(media.FileLength),
			Caption:       proto.String(caption),
		},
	})
	if err != nil {
		return fmt.Errorf("échec d'envoi de l'image: %w", err)
	}
	return nil
}

// SendDocument envoie un document avec nom de fichier et légende optionnelle.
func (p *Provider) SendDocument(ctx context.Context, to string, data []byte, filename, caption string) error {
	jid, err := toJID(to)
	if err != nil {
		return err
	}

	media, err := p.client.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return fmt.Errorf("échec d'upload du document: %w", err)
	}

	_, err = p.client.SendMessage(ctx, jid, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			URL:           proto.String(media.URL),
			DirectPath:    proto.String(media.DirectPath),
			FileLength:    proto.Uint64(media.FileLength),
			FileName:      proto.String(filename),
			Caption:       proto.String(caption),
			MediaKey:      media.MediaKey,
			FileEncSHA256: media.FileEncSHA256,
			FileSHA256:    media.FileSHA256,
		},
	})
	if err != nil {
		return fmt.Errorf("échec d'envoi du document: %w", err)
	}
	return nil
}

// SetAutoRejectCalls active ou désactive le rejet automatique des appels
// entrants (voir internal/whatsapp/meow/calls.go pour la logique associée).
func (p *Provider) SetAutoRejectCalls(enabled bool) {
	p.mu.Lock()
	p.autoRejectCalls = enabled
	p.mu.Unlock()
}

func toJID(to string) (types.JID, error) {
	// Un identifiant déjà complet (numéro individuel "...@s.whatsapp.net"
	// ou groupe "...@g.us") est utilisé tel quel — utile pour répondre
	// directement dans un groupe (voir internal/moderation). Un numéro brut
	// (sans "@") est traité comme un contact individuel.
	if strings.Contains(to, "@") {
		jid, err := types.ParseJID(to)
		if err != nil {
			return types.JID{}, fmt.Errorf("jid invalide %q: %w", to, err)
		}
		return jid, nil
	}
	jid, err := types.ParseJID(to + "@s.whatsapp.net")
	if err != nil {
		return types.JID{}, fmt.Errorf("numéro invalide %q: %w", to, err)
	}
	return jid, nil
}
