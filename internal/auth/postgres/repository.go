// Package postgres implémente auth.Repository avec PostgreSQL.
//
// IMPORTANT : ce paquet importe le driver PostgreSQL
// (github.com/jackc/pgx/v5/stdlib), qui n'est PAS présent dans go.mod/go.sum
// de ce projet — impossible de l'ajouter dans l'environnement où ce code a
// été écrit (pas d'accès réseau). Ce paquet ne compilera donc PAS tel quel.
//
// Pour l'activer :
//  1. go get github.com/jackc/pgx/v5/stdlib
//  2. Dans cmd/server/main.go, remplacer l'import et l'appel à
//     authsqlite.New(cfg.AuthDBPath) par postgres.New(cfg.AuthDBURL)
//  3. Définir AUTH_DB_URL (ex: postgres://user:pass@localhost:5432/overline)
//
// Ce paquet n'est importé nulle part par défaut — tant que tu ne le branches
// pas dans main.go, il n'empêche pas `go build ./cmd/server` de fonctionner.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"overline-connect/internal/auth"
)

// Repository implémente auth.Repository avec PostgreSQL.
type Repository struct {
	db *sql.DB
}

// New ouvre une connexion PostgreSQL à l'URL donnée et s'assure que le
// schéma existe.
func New(databaseURL string) (*Repository, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("échec d'ouverture de la connexion postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("échec de connexion à postgres: %w", err)
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
			created_at    TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS api_keys (
			id         TEXT PRIMARY KEY,
			company_id TEXT NOT NULL REFERENCES companies(id),
			key_hash   TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("échec de migration du schéma auth: %w", err)
	}
	return nil
}

func (r *Repository) CreateCompany(ctx context.Context, c auth.Company) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO companies (id, name, email, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.Name, c.Email, c.PasswordHash, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("échec d'insertion de l'entreprise: %w", err)
	}
	return nil
}

func (r *Repository) GetCompanyByEmail(ctx context.Context, email string) (auth.Company, error) {
	return r.scanCompany(r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at FROM companies WHERE email = $1`, email))
}

func (r *Repository) GetCompanyByID(ctx context.Context, id string) (auth.Company, error) {
	return r.scanCompany(r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at FROM companies WHERE id = $1`, id))
}

func (r *Repository) CreateAPIKey(ctx context.Context, k auth.APIKey) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO api_keys (id, company_id, key_hash, created_at) VALUES ($1, $2, $3, $4)`,
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
		WHERE k.key_hash = $1
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
