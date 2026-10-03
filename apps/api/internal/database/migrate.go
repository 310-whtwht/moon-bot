package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// baselineVersions are the migrations that were applied by the MySQL
// docker-entrypoint-initdb.d mount before this migrator existed.
var baselineVersions = []string{
	"001_create_strategy_packages.sql",
	"002_create_orders_trades.sql",
	"003_create_audit_logs_universe.sql",
	"004_create_trade_traces.sql",
	"005_create_backtests.sql",
	"006_seed_data.sql",
}

// Migrate applies every embedded migration that is not yet recorded in
// schema_migrations. The db connection must be opened with multiStatements=true.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`); err != nil {
		return fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	if err := baselineIfNeeded(ctx, db); err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	versions, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("failed to list migrations: %w", err)
	}
	sort.Strings(versions)

	for _, path := range versions {
		version := strings.TrimPrefix(path, "migrations/")
		if applied[version] {
			continue
		}

		body, err := migrationFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", version, err)
		}

		log.Printf("Applying migration %s", version)
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", version, err)
		}
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}
	}

	return nil
}

// baselineIfNeeded records the initdb-era migrations as applied when the
// schema already exists but schema_migrations is empty, so they are not re-run.
func baselineIfNeeded(ctx context.Context, db *sql.DB) error {
	var recorded int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&recorded); err != nil {
		return fmt.Errorf("failed to count schema_migrations: %w", err)
	}
	if recorded > 0 {
		return nil
	}

	var existing int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'strategy_packages'",
	).Scan(&existing); err != nil {
		return fmt.Errorf("failed to inspect existing schema: %w", err)
	}
	if existing == 0 {
		return nil
	}

	log.Println("Existing schema detected; recording initdb migrations as applied")
	for _, version := range baselineVersions {
		if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record baseline %s: %w", version, err)
		}
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}
