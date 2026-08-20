// Package settings gère les paramètres de modération par entreprise
// (antilien, anti-appel, etc. — inspirés des commandes Toxic-MD).
package settings

import "context"

// CompanySettings regroupe les paramètres activables pour une entreprise.
type CompanySettings struct {
	CompanyID       string
	AntilinkEnabled bool
	AnticallEnabled bool
}

// Repository est le contrat de stockage des paramètres par entreprise.
type Repository interface {
	// Get retourne les paramètres de l'entreprise. Si aucun paramètre n'a
	// encore été enregistré, retourne des valeurs par défaut (tout
	// désactivé) sans erreur.
	Get(ctx context.Context, companyID string) (CompanySettings, error)
	Save(ctx context.Context, s CompanySettings) error
}
