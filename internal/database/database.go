package database

import (
	"context"
	"corpord-api/internal/config"
	"time"
)

// Database объединяет все хранилища приложения
type Database struct {
	Postgres Pg
	Redis    *Rd
}

// New создает новый экземпляр Database
func New(ctx context.Context, cfg *config.Database) *Database {
	return &Database{
		Postgres: NewPostgres(ctx, &cfg.Postgres),
		Redis:    NewRedis(&cfg.Redis),
	}
}

// Ping проверяет соединение со всеми базами данных
func (db *Database) Ping(ctx context.Context) error {
	if err := db.Postgres.Ping(ctx); err != nil {
		return err
	}

	if db.Redis != nil {
		if err := db.Redis.Ping(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Close закрывает все соединения с базами данных
func (db *Database) Close() {
	if db.Postgres != nil {
		db.Postgres.Close()
	}

	if db.Redis != nil {
		_ = db.Redis.Close()
	}
}

// HealthCheck возвращает статус работоспособности базы данных
type HealthCheck struct {
	Postgres bool          `json:"postgres"`
	Redis    bool          `json:"redis"`
	Uptime   time.Duration `json:"uptime"`
}

// Health возвращает статус работоспособности всех баз данных
func (db *Database) Health(ctx context.Context) HealthCheck {
	health := HealthCheck{
		Postgres: db.Postgres.Ping(ctx) == nil,
	}

	if db.Redis != nil {
		health.Redis = db.Redis.Ping(ctx) == nil
	}

	return health
}
