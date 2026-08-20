package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword hache un mot de passe avec bcrypt (coût par défaut).
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("échec de hachage du mot de passe: %w", err)
	}
	return string(hash), nil
}

// checkPassword vérifie un mot de passe en clair contre un hash bcrypt.
func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// generateAPIKey crée une nouvelle clé API en clair, préfixée pour être
// facilement identifiable (ex: dans des logs, par erreur).
func generateAPIKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("échec de génération de la clé API: %w", err)
	}
	return "olc_" + hex.EncodeToString(raw), nil
}

// hashAPIKey hache une clé API en clair (SHA-256, suffisant ici car la clé
// elle-même a déjà une entropie élevée — contrairement à un mot de passe).
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
