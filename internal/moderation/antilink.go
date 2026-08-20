// Package moderation contient la logique de modération des conversations
// (antilien pour l'instant — inspiré des fonctionnalités Toxic-MD, adapté à
// l'architecture multi-entreprises d'OverLine Connect).
package moderation

import (
	"regexp"
	"strings"
)

var linkPattern = regexp.MustCompile(`(?i)(https?://\S+|www\.\S+|chat\.whatsapp\.com/\S+)`)

// ContainsLink retourne vrai si le texte contient une URL ou un lien
// d'invitation de groupe WhatsApp.
func ContainsLink(text string) bool {
	return linkPattern.MatchString(text)
}

// IsGroupChat retourne vrai si l'identifiant de conversation (ChatJID)
// désigne un groupe WhatsApp plutôt qu'une conversation individuelle.
func IsGroupChat(chatJID string) bool {
	return strings.HasSuffix(chatJID, "@g.us")
}
