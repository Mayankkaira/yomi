package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Run applies all embedded SQL migrations for a service's schema against the
// database at url. It is idempotent and safe to call on every startup. A
// dedicated database/sql connection is opened because the migrate/pgx driver
// requires the stdlib interface rather than pgxpool.
func Run(url string, fs embed.FS, dir string) error {
	src, err := iofs.New(fs, dir)
	if err != nil {
		return fmt.Errorf("open embedded migrations %s: %w", dir, err)
	}
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer sqlDB.Close()

	driver, err := pgx.WithInstance(sqlDB, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("pgx migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx", driver)
	if err != nil {
		return fmt.Errorf("migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations %s: %w", dir, err)
	}
	slog.Info("migrations applied", "dir", dir)
	return nil
}
