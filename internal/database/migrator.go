package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
)

type Migrator struct {
	db     *sql.DB
	dbName string
}

func NewMigrator(db *sql.DB, dbName string) *Migrator {
	return &Migrator{
		db:     db,
		dbName: dbName,
	}
}

func (m *Migrator) MigrateUp(ctx context.Context) error {
	migrateCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.RunContext(migrateCtx, "up", m.db, "./db/migrations"); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("migration timed out: %w", err)
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}
