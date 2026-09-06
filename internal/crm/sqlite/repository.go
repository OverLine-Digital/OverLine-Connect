// Package sqlite implémente crm.Repository avec SQLite.
//
// Isolation multi-tenant : une seule base partagée, chaque ligne portant un
// company_id, filtré systématiquement dans chaque requête. C'est un choix
// différent de internal/whatsapp (un fichier de session par entreprise) et
// internal/auth (un seul fichier global) — les trois approches sont
// valables ; celle-ci convient bien à des données qui grandissent en
// nombre de lignes plutôt qu'en fichiers séparés (messages).
//
// TEMPORAIRE comme les autres paquets sqlite du projet : à remplacer par
// PostgreSQL pour un vrai déploiement à grande échelle (voir
// internal/auth/postgres pour le même type de migration déjà préparée).
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"overline-connect/internal/crm"
)

// Repository implémente crm.Repository avec SQLite.
type Repository struct {
	db *sql.DB
}

// New ouvre (ou crée) la base SQLite au chemin donné et s'assure que le
// schéma existe.
func New(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_foreign_keys=on&_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("échec d'ouverture de la base crm: %w", err)
	}

	// Une seule connexion à la fois : SQLite n'autorise qu'un seul
	// écrivain simultané de toute façon, et laisser Go ouvrir plusieurs
	// connexions en parallèle ne fait qu'aggraver les blocages
	// ("database is locked") observés lors de l'import massif de
	// l'historique WhatsApp (plusieurs milliers de messages d'un coup).
	db.SetMaxOpenConns(1)

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *Repository) migrate() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id         TEXT PRIMARY KEY,
			company_id TEXT NOT NULL,
			chat_jid   TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			text       TEXT NOT NULL,
			from_me    INTEGER NOT NULL,
			timestamp  DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_messages_company_chat
			ON messages (company_id, chat_jid, timestamp);
	`)
	if err != nil {
		return fmt.Errorf("échec de migration du schéma crm: %w", err)
	}
	return nil
}

func (r *Repository) SaveMessage(ctx context.Context, m crm.Message) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO messages (id, company_id, chat_jid, sender_jid, text, from_me, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.CompanyID, m.ChatJID, m.SenderJID, m.Text, m.FromMe, m.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("échec d'insertion du message: %w", err)
	}
	return nil
}

func (r *Repository) ListConversations(ctx context.Context, companyID string) ([]crm.Conversation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT chat_jid, MAX(timestamp) AS last_at
		FROM messages
		WHERE company_id = ?
		GROUP BY chat_jid
		ORDER BY last_at DESC
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("échec de lecture des conversations: %w", err)
	}

	type row struct {
		chatJID string
		lastAt  time.Time
	}
	var collected []row
	for rows.Next() {
		var rr row
		if err := rows.Scan(&rr.chatJID, &rr.lastAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("échec de lecture d'une conversation: %w", err)
		}
		collected = append(collected, rr)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	// IMPORTANT : on ferme rows explicitement ICI, avant d'exécuter
	// d'autres requêtes (lastMessageText ci-dessous). Avec une seule
	// connexion autorisée vers la base (SetMaxOpenConns(1)), garder rows
	// ouvert pendant qu'on lance une nouvelle requête bloquait
	// indéfiniment — c'est ce qui empêchait l'Inbox de jamais rien
	// afficher malgré des messages bien enregistrés.
	rows.Close()

	conversations := make([]crm.Conversation, 0, len(collected))
	for _, rr := range collected {
		lastText, err := r.lastMessageText(ctx, companyID, rr.chatJID)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, crm.Conversation{
			ChatJID:         rr.chatJID,
			CompanyID:       companyID,
			LastMessageAt:   rr.lastAt,
			LastMessageText: lastText,
		})
	}
	return conversations, nil
}

func (r *Repository) lastMessageText(ctx context.Context, companyID, chatJID string) (string, error) {
	var text string
	err := r.db.QueryRowContext(ctx, `
		SELECT text FROM messages
		WHERE company_id = ? AND chat_jid = ?
		ORDER BY timestamp DESC LIMIT 1
	`, companyID, chatJID).Scan(&text)
	if err != nil {
		return "", fmt.Errorf("échec de lecture du dernier message: %w", err)
	}
	return text, nil
}

// CountMessages retourne le nombre total de messages enregistrés pour une
// entreprise, tous chats confondus. Route de diagnostic temporaire — à
// retirer une fois le bug d'affichage de l'Inbox confirmé résolu.
func (r *Repository) CountMessages(ctx context.Context, companyID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE company_id = ?`, companyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("échec de comptage des messages: %w", err)
	}
	return count, nil
}

func (r *Repository) ListMessages(ctx context.Context, companyID, chatJID string, limit int) ([]crm.Message, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, chat_jid, sender_jid, text, from_me, timestamp
		FROM (
			SELECT id, company_id, chat_jid, sender_jid, text, from_me, timestamp
			FROM messages
			WHERE company_id = ? AND chat_jid = ?
			ORDER BY timestamp DESC
			LIMIT ?
		)
		ORDER BY timestamp ASC
	`, companyID, chatJID, limit)
	if err != nil {
		return nil, fmt.Errorf("échec de lecture des messages: %w", err)
	}
	defer rows.Close()

	var messages []crm.Message
	for rows.Next() {
		var m crm.Message
		if err := rows.Scan(&m.ID, &m.CompanyID, &m.ChatJID, &m.SenderJID, &m.Text, &m.FromMe, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("échec de lecture d'un message: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
