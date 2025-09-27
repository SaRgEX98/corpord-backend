package database

import (
	"context"
	"corpord-api/internal/config"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Rd struct {
	client *redis.Client
}

// NewRedis создает новый экземпляр клиента Redis
func NewRedis(cfg *config.Redis) *Rd {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to ping redis: %v", err)
	}

	return &Rd{client: client}
}

// Ping проверяет соединение с Redis
func (r *Rd) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close закрывает соединение с Redis
func (r *Rd) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Client возвращает клиент Redis
func (r *Rd) Client() *redis.Client {
	return r.client
}
