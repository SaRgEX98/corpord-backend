package database

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"corpord-api/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

type Pg interface {
	MigrateUp(ctx context.Context) error
	Ping(ctx context.Context) error
	Close()
	Pool() *pgxpool.Pool
	DB() *sqlx.DB
}

type pg struct {
	pool *pgxpool.Pool
	db   *sqlx.DB
	name string
}

func NewPostgres(ctx context.Context, cfg *config.Postgres) Pg {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		log.Fatalf("failed to parse DSN: %v", err)
	}

	// Настройка пула соединений
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}

	// Проверяем соединение
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	return &pg{
		pool: pool,
		name: cfg.DBName,
		db:   db,
	}
}

func (p *pg) MigrateUp(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(p.pool)
	defer db.Close()

	migrator := NewMigrator(db, p.name)
	return migrator.MigrateUp(ctx)
}

func (p *pg) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *pg) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
	if p.db != nil {
		p.db.Close()
	}
}

func (p *pg) Pool() *pgxpool.Pool {
	return p.pool
}

func (p *pg) DB() *sqlx.DB {
	return p.db
}
