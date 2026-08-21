// Package sqlite implémente settings.Repository avec SQLite (pur Go, sans
// cgo — cohérent avec le reste du projet, voir internal/auth/sqlite).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"

	"overline-connect/internal/settings"
)

// Repository implémente settings.Repository avec SQLite.
type Repository struct {
	db *sql.DB
}

// New ouvre (ou crée) la base SQLite au chemin donné et s'assure que le
// schéma existe.
func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_foreign_keys=on&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, fmt.Errorf("échec d'ouverture de la base settings: %w", err)
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS company_settings (
			company_id       TEXT PRIMARY KEY,
			antilink_enabled INTEGER NOT NULL DEFAULT 0,
			anticall_enabled INTEGER NOT NULL DEFAULT 0
		);
	`)
	if err != nil {
		return fmt.Errorf("échec de migration du schéma settings: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, companyID string) (settings.CompanySettings, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT antilink_enabled, anticall_enabled FROM company_settings WHERE company_id = ?`, companyID)

	var antilink, anticall bool
	err := row.Scan(&antilink, &anticall)
	if errors.Is(err, sql.ErrNoRows) {
		// Pas encore de paramètres enregistrés : valeurs par défaut.
		return settings.CompanySettings{CompanyID: companyID}, nil
	}
	if err != nil {
		return settings.CompanySettings{}, fmt.Errorf("échec de lecture des paramètres: %w", err)
	}

	return settings.CompanySettings{
		CompanyID:       companyID,
		AntilinkEnabled: antilink,
		AnticallEnabled: anticall,
	}, nil
}

func (r *Repository) Save(ctx context.Context, s settings.CompanySettings) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO company_settings (company_id, antilink_enabled, anticall_enabled)
		VALUES (?, ?, ?)
		ON CONFLICT(company_id) DO UPDATE SET
			antilink_enabled = excluded.antilink_enabled,
			anticall_enabled = excluded.anticall_enabled
	`, s.CompanyID, s.AntilinkEnabled, s.AnticallEnabled)
	if err != nil {
		return fmt.Errorf("échec d'enregistrement des paramètres: %w", err)
	}
	return nil
}
