package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteStore struct {
	db *sql.DB
}

func openStore(path string) (*sqliteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) ensureMigrationTable() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS _schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

func (s *sqliteStore) appliedMigrations() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT name FROM _schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func (s *sqliteStore) applyMigrationStatement(statement string) error {
	_, err := s.db.Exec(statement)
	return err
}

func (s *sqliteStore) recordMigration(name string) error {
	_, err := s.db.Exec(`INSERT INTO _schema_migrations (name, applied_at) VALUES (?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *sqliteStore) seedAdmin(username, password string) error {
	if username == "" || password == "" {
		return errors.New("admin username and password required")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE username = ?`, username).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO admins (username, password_hash, created_at) VALUES (?, ?, ?)`, username, hash, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *sqliteStore) verifyAdmin(username, password string) (bool, error) {
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM admins WHERE username = ?`, username).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return checkPassword(hash, password), nil
}

func (s *sqliteStore) createAPIKey(label string) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := hex.EncodeToString(buf)
	hash, err := hashPassword("key:" + raw)
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(`INSERT INTO api_keys (key_hash, label, created_at) VALUES (?, ?, ?)`, hash, label, time.Now().UTC().Format(time.RFC3339))
	return raw, err
}

func (s *sqliteStore) validAPIKey(raw string) (bool, error) {
	rows, err := s.db.Query(`SELECT key_hash, revoked FROM api_keys`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var hash string
		var revoked int
		if err := rows.Scan(&hash, &revoked); err != nil {
			return false, err
		}
		if revoked == 0 {
			ok, err := checkPasswordHash(hash, "key:"+raw)
			if err == nil && ok {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

func (s *sqliteStore) createTxn(reference, amount, merchant, qris string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO transactions (reference, amount, merchant, qris, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)`, reference, amount, merchant, qris, now, now)
	return err
}

func (s *sqliteStore) updateTxnStatus(reference, status, provider, providerRef string) (bool, error) {
	res, err := s.db.Exec(`UPDATE transactions SET status = ?, provider = ?, provider_ref = ?, updated_at = ? WHERE reference = ?`,
		status, provider, providerRef, time.Now().UTC().Format(time.RFC3339), reference)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *sqliteStore) getTxn(reference string) (*txnRow, error) {
	row := s.db.QueryRow(`SELECT id, reference, amount, merchant, status, provider, provider_ref, created_at, updated_at FROM transactions WHERE reference = ?`, reference)
	t := &txnRow{}
	err := row.Scan(&t.ID, &t.Reference, &t.Amount, &t.Merchant, &t.Status, &t.Provider, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (s *sqliteStore) listTxns(limit int) ([]txnRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, reference, amount, merchant, status, provider, provider_ref, created_at, updated_at FROM transactions ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []txnRow
	for rows.Next() {
		t := txnRow{}
		if err := rows.Scan(&t.ID, &t.Reference, &t.Amount, &t.Merchant, &t.Status, &t.Provider, &t.ProviderRef, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// hashPassword prefixes a SHA-256 hex digest. For production, swap for bcrypt.
func hashPassword(password string) (string, error) {
	return "sha256:" + sha256Hex(password), nil
}

func checkPassword(hash, password string) bool {
	return subtle.ConstantTimeCompare([]byte(hash), []byte("sha256:"+sha256Hex(password))) == 1
}

func checkPasswordHash(hash, password string) (bool, error) {
	return subtle.ConstantTimeCompare([]byte(hash), []byte("sha256:"+sha256Hex(password))) == 1, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
