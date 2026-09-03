package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type migrationBackend interface {
	ensureMigrationTable() error
	appliedMigrations() (map[string]bool, error)
	applyMigrationStatement(statement string) error
	recordMigration(name string) error
}

func findMigrationsDir() string {
	if configured := os.Getenv("MIGRATIONS_DIR"); configured != "" {
		return configured
	}
	dir, err := os.Getwd()
	if err != nil {
		return "migrations"
	}
	for depth := 0; depth < 6; depth++ {
		candidate := filepath.Join(dir, "migrations")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "migrations"
}

func runMigrations(dir string, backend migrationBackend) error {
	if err := backend.ensureMigrationTable(); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	applied, err := backend.appliedMigrations()
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}

	for _, file := range files {
		name := strings.TrimSuffix(file, ".up.sql")
		if applied[name] {
			continue
		}
		content, err := os.ReadFile(dir + "/" + file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		for _, statement := range splitSQLStatements(string(content)) {
			if err := backend.applyMigrationStatement(statement); err != nil {
				return fmt.Errorf("apply migration %s: %w", file, err)
			}
		}
		if err := backend.recordMigration(name); err != nil {
			return fmt.Errorf("record migration %s: %w", file, err)
		}
	}
	return nil
}

func splitSQLStatements(sqlText string) []string {
	var statements []string
	var current strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		if ch == '\'' && !inDouble {
			if inSingle && i+1 < len(sqlText) && sqlText[i+1] == '\'' {
				current.WriteByte(ch)
				current.WriteByte(sqlText[i+1])
				i++
				continue
			}
			inSingle = !inSingle
		} else if ch == '"' && !inSingle {
			inDouble = !inDouble
		}
		if ch == ';' && !inSingle && !inDouble {
			if statement := strings.TrimSpace(current.String()); statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	if statement := strings.TrimSpace(current.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
