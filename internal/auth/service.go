package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service contient la logique métier d'authentification. Il ne dépend que
// de l'interface Repository, jamais d'un moteur de base de données précis.
type Service struct {
	repo Repository
}

// NewService crée un Service à partir d'un Repository.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SignUp crée une nouvelle entreprise et sa première clé API. La clé en
// clair n'est retournée qu'ici — elle n'est jamais stockée ni récupérable
// ensuite, seul son hash l'est.
func (s *Service) SignUp(ctx context.Context, name, email, password string) (Company, string, error) {
	if _, err := s.repo.GetCompanyByEmail(ctx, email); err == nil {
		return Company{}, "", ErrDuplicateEmail
	} else if err != ErrNotFound {
		return Company{}, "", err
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return Company{}, "", err
	}

	company := Company{
		ID:           uuid.New().String(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	if err := s.repo.CreateCompany(ctx, company); err != nil {
		return Company{}, "", fmt.Errorf("échec de création de l'entreprise: %w", err)
	}

	apiKey, err := s.issueAPIKey(ctx, company.ID)
	if err != nil {
		return Company{}, "", err
	}

	return company, apiKey, nil
}

// Authenticate vérifie un email et mot de passe, et retourne l'entreprise
// correspondante si valides.
func (s *Service) Authenticate(ctx context.Context, email, password string) (Company, error) {
	company, err := s.repo.GetCompanyByEmail(ctx, email)
	if err != nil {
		return Company{}, err
	}
	if !checkPassword(company.PasswordHash, password) {
		return Company{}, ErrNotFound
	}
	return company, nil
}

// VerifyAPIKey retrouve l'entreprise propriétaire d'une clé API en clair.
// Utilisé par le middleware HTTP à chaque requête authentifiée.
func (s *Service) VerifyAPIKey(ctx context.Context, plainKey string) (Company, error) {
	return s.repo.GetCompanyByAPIKeyHash(ctx, hashAPIKey(plainKey))
}

// IssueAPIKey génère une nouvelle clé API pour une entreprise existante
// (ex: rotation de clé, clé additionnelle pour une intégration).
func (s *Service) IssueAPIKey(ctx context.Context, companyID string) (string, error) {
	return s.issueAPIKey(ctx, companyID)
}

func (s *Service) issueAPIKey(ctx context.Context, companyID string) (string, error) {
	plainKey, err := generateAPIKey()
	if err != nil {
		return "", err
	}

	key := APIKey{
		ID:        uuid.New().String(),
		CompanyID: companyID,
		KeyHash:   hashAPIKey(plainKey),
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateAPIKey(ctx, key); err != nil {
		return "", fmt.Errorf("échec de création de la clé API: %w", err)
	}

	return plainKey, nil
}
