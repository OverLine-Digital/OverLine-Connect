// Package sqlite implémente auth.Repository avec SQLite.
//
// TEMPORAIRE : SQLite convient au développement solo et aux tests, mais
// n'est pas adapté à un vrai déploiement multi-entreprises (pas d'accès
// concurrent robuste, pas de serveur réseau séparé). La migration prévue
// est PostgreSQL — il suffira d'écrire un nouveau paquet
// internal/auth/postgres implémentant auth.Repository, et de changer une
// ligne dans cmd/server/main.go. Rien d'autre à modifier.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"overline-connect/internal/auth"
)

// Repository implémente auth.Repository avec SQLite.
type Repository struct {
	db *sql.DB
}

// New ouvre (ou crée) la base SQLite au chemin donné et s'assure que le
// schéma existe.
func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("échec d'ouverture de la base auth: %w", err)
	}

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS companies (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			email         TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS api_keys (
			id         TEXT PRIMARY KEY,
			company_id TEXT NOT NULL REFERENCES companies(id),
			key_hash   TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("échec de migration du schéma auth: %w", err)
	}
	return nil
}

func (r *Repository) CreateCompany(ctx context.Context, c auth.Company) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO companies (id, name, email, password_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Email, c.PasswordHash, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("échec d'insertion de l'entreprise: %w", err)
	}
	return nil
}

func (r *Repository) GetCompanyByEmail(ctx context.Context, email string) (auth.Company, error) {
	return r.scanCompany(r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at FROM companies WHERE email = ?`, email))
}

func (r *Repository) GetCompanyByID(ctx context.Context, id string) (auth.Company, error) {
	return r.scanCompany(r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at FROM companies WHERE id = ?`, id))
}

func (r *Repository) CreateAPIKey(ctx context.Context, k auth.APIKey) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, company_id, key_hash, created_at) VALUES (?, ?, ?, ?)`,
		k.ID, k.CompanyID, k.KeyHash, k.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("échec d'insertion de la clé API: %w", err)
	}
	return nil
}

func (r *Repository) GetCompanyByAPIKeyHash(ctx context.Context, keyHash string) (auth.Company, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.email, c.password_hash, c.created_at
		FROM companies c
		JOIN api_keys k ON k.company_id = c.id
		WHERE k.key_hash = ?
	`, keyHash)
	return r.scanCompany(row)
}

func (r *Repository) scanCompany(row *sql.Row) (auth.Company, error) {
	var c auth.Company
	var createdAt time.Time
	err := row.Scan(&c.ID, &c.Name, &c.Email, &c.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Company{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.Company{}, fmt.Errorf("échec de lecture de l'entreprise: %w", err)
	}
	c.CreatedAt = createdAt
	return c, nil
}
