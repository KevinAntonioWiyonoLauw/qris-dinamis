package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func randomBytes(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	return buf, err
}

func hexEncode(b []byte) string { return hex.EncodeToString(b) }

// d1store persists through the Cloudflare D1 REST API.
type d1store struct {
	accountID  string
	databaseID string
	apiToken   string
	client     *http.Client
}

type d1Config struct {
	AccountID  string
	DatabaseID string
	APIToken   string
}

func openD1Store(cfg d1Config) (*d1store, error) {
	if cfg.AccountID == "" || cfg.DatabaseID == "" || cfg.APIToken == "" {
		return nil, errors.New("D1_ACCOUNT_ID, D1_DATABASE_ID and CLOUDFLARE_API_TOKEN are required")
	}
	s := &d1store{
		accountID:  cfg.AccountID,
		databaseID: cfg.DatabaseID,
		apiToken:   cfg.APIToken,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
	return s, nil
}

func (s *d1store) ensureMigrationTable() error {
	return s.exec(`CREATE TABLE IF NOT EXISTS _schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
}

func (s *d1store) appliedMigrations() (map[string]bool, error) {
	rows, err := s.query(`SELECT name FROM _schema_migrations`)
	if err != nil {
		return nil, err
	}
	applied := map[string]bool{}
	for _, row := range rows {
		applied[strVal(row["name"])] = true
	}
	return applied, nil
}

func (s *d1store) applyMigrationStatement(statement string) error {
	return s.exec(statement)
}

func (s *d1store) recordMigration(name string) error {
	return s.exec(`INSERT INTO _schema_migrations (name, applied_at) VALUES (?, ?)`, name, time.Now().UTC().Format(time.RFC3339))
}

func (s *d1store) url() string {
	return fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query", s.accountID, s.databaseID)
}

type d1Response struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result []struct {
		Results  []map[string]any `json:"results"`
		Success  bool             `json:"success"`
		Meta     map[string]any   `json:"meta"`
		ErrorMsg string           `json:"error"`
	} `json:"result"`
}

func (s *d1store) query(sql string, params ...any) ([]map[string]any, error) {
	body, err := json.Marshal(map[string]any{"sql": sql, "params": params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, s.url(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out d1Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Success {
		msg := "d1 error"
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return nil, errors.New(msg)
	}
	if len(out.Result) == 0 {
		return nil, nil
	}
	if msg := out.Result[0].ErrorMsg; msg != "" {
		return nil, errors.New(msg)
	}
	return out.Result[0].Results, nil
}

func (s *d1store) exec(sql string, params ...any) error {
	_, err := s.query(sql, params...)
	return err
}

func (s *d1store) seedAdmin(username, password string) error {
	if username == "" || password == "" {
		return errors.New("admin username and password required")
	}
	rows, err := s.query(`SELECT COUNT(*) AS n FROM admins WHERE username = ?`, username)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		if n, ok := rows[0]["n"]; ok {
			if f, ok := n.(float64); ok && f > 0 {
				return nil
			}
		}
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return s.exec(`INSERT INTO admins (username, password_hash, created_at) VALUES (?, ?, ?)`, username, hash, time.Now().UTC().Format(time.RFC3339))
}

func (s *d1store) verifyAdmin(username, password string) (bool, error) {
	rows, err := s.query(`SELECT password_hash FROM admins WHERE username = ?`, username)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 {
		return false, nil
	}
	hash, _ := rows[0]["password_hash"].(string)
	return checkPassword(hash, password), nil
}

func (s *d1store) createAPIKey(label string) (string, error) {
	buf, err := randomBytes(24)
	if err != nil {
		return "", err
	}
	raw := hexEncode(buf)
	hash, err := hashPassword("key:" + raw)
	if err != nil {
		return "", err
	}
	if err := s.exec(`INSERT INTO api_keys (key_hash, label, created_at) VALUES (?, ?, ?)`, hash, label, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *d1store) validAPIKey(raw string) (bool, error) {
	rows, err := s.query(`SELECT key_hash, revoked FROM api_keys`)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		hash, _ := row["key_hash"].(string)
		revoked, _ := row["revoked"].(float64)
		if revoked == 0 {
			if ok, err := checkPasswordHash(hash, "key:"+raw); err == nil && ok {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *d1store) createTxn(reference, amount, merchant, qris string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return s.exec(`INSERT INTO transactions (reference, amount, merchant, qris, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)`, reference, amount, merchant, qris, now, now)
}

func (s *d1store) updateTxnStatus(reference, status, provider, providerRef string) (bool, error) {
	_, err := s.query(`UPDATE transactions SET status = ?, provider = ?, provider_ref = ?, updated_at = ? WHERE reference = ?`,
		status, provider, providerRef, time.Now().UTC().Format(time.RFC3339), reference)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *d1store) getTxn(reference string) (*txnRow, error) {
	rows, err := s.query(`SELECT id, reference, amount, merchant, status, provider, provider_ref, created_at, updated_at FROM transactions WHERE reference = ?`, reference)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &txnRow{
		ID:          int64(row["id"].(float64)),
		Reference:   strVal(row["reference"]),
		Amount:      strVal(row["amount"]),
		Merchant:    strVal(row["merchant"]),
		Status:      strVal(row["status"]),
		Provider:    strVal(row["provider"]),
		ProviderRef: strVal(row["provider_ref"]),
		CreatedAt:   strVal(row["created_at"]),
		UpdatedAt:   strVal(row["updated_at"]),
	}, nil
}

func (s *d1store) listTxns(limit int) ([]txnRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.query(`SELECT id, reference, amount, merchant, status, provider, provider_ref, created_at, updated_at FROM transactions ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var out []txnRow
	for _, row := range rows {
		out = append(out, txnRow{
			ID:          int64(row["id"].(float64)),
			Reference:   strVal(row["reference"]),
			Amount:      strVal(row["amount"]),
			Merchant:    strVal(row["merchant"]),
			Status:      strVal(row["status"]),
			Provider:    strVal(row["provider"]),
			ProviderRef: strVal(row["provider_ref"]),
			CreatedAt:   strVal(row["created_at"]),
			UpdatedAt:   strVal(row["updated_at"]),
		})
	}
	return out, nil
}

func strVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}
