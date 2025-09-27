package main

import (
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/database"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	go func() {
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.Canceled) {
			log.Println("Shutdown signal received, canceling...")
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.Println("Timeout reached, canceling...")
		}
	}()

	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db := database.New(ctx, &cfg.Database)
	defer db.Close()

	log.Println("Starting database migrations...")
	if err := db.Postgres.MigrateUp(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Fatal("Migration timed out. Please check if the database is accessible and try again.")
		}
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	log.Println("Migrations applied successfully")
}
