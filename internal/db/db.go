package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(databasePath string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(databasePath) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := Migrate(context.Background(), conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// CurrentSchemaVersion возвращает MAX(version) из schema_migrations.
func CurrentSchemaVersion(ctx context.Context, conn *sql.DB) (int, error) {
	var v int
	if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var current int
	row := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&current); err != nil {
		return err
	}

	for _, fname := range files {
		v, err := strconv.Atoi(strings.Split(fname, "_")[0])
		if err != nil {
			continue
		}
		if v <= current {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + fname)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", fname, err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", fname, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, v); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
