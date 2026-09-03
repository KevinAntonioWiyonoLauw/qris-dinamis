package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestRunMigrationsIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "qris.db")
	local, err := openStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer local.db.Close()

	if err := runMigrations(filepath.Join("..", "..", "migrations"), local); err != nil {
		t.Fatal(err)
	}
	if err := runMigrations(filepath.Join("..", "..", "migrations"), local); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := local.db.QueryRow(`SELECT COUNT(*) FROM _schema_migrations WHERE name = '000001_initial'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration records = %d, want 1", count)
	}
	for _, table := range []string{"admins", "api_keys", "transactions"} {
		var name string
		if err := local.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			if err == sql.ErrNoRows {
				t.Fatalf("table %s missing", table)
			}
			t.Fatal(err)
		}
	}
}
