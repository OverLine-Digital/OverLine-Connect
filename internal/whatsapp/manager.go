package whatsapp

import (
	"fmt"
	"sync"
)

// Factory construit un nouveau Provider pour une entreprise donnée (par
// exemple, un Provider whatsmeow avec un fichier de session dédié à cette
// entreprise).
type Factory func(companyID string) (Provider, error)

// Manager garde en mémoire un Provider par entreprise, créé à la demande via
// Factory. C'est ce qui permet à chaque entreprise d'avoir sa propre session
// WhatsApp, isolée des autres.
//
// LIMITE CONNUE : l'état vit uniquement en mémoire du process. Un redémarrage
// du serveur oblige chaque Provider à se recréer (la session whatsmeow,
// elle, persiste sur disque et se reconnecte normalement — mais un
// Provider "en attente de scan QR" au moment du redémarrage perd cet état et
// doit relancer Connect()).
type Manager struct {
	mu        sync.Mutex
	providers map[string]Provider
	factory   Factory
}

// NewManager crée un Manager utilisant la Factory donnée pour instancier de
// nouveaux Provider.
func NewManager(factory Factory) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		factory:   factory,
	}
}

// GetOrCreate retourne le Provider de l'entreprise donnée, en le créant
// (sans le connecter) s'il n'existe pas encore.
func (m *Manager) GetOrCreate(companyID string) (Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.providers[companyID]; ok {
		return p, nil
	}

	p, err := m.factory(companyID)
	if err != nil {
		return nil, fmt.Errorf("échec de création du provider whatsapp pour l'entreprise %s: %w", companyID, err)
	}
	m.providers[companyID] = p
	return p, nil
}

// Get retourne le Provider existant de l'entreprise donnée, s'il a déjà été
// créé (ne le crée jamais).
func (m *Manager) Get(companyID string) (Provider, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.providers[companyID]
	return p, ok
}
