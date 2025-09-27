package app

import (
	"context"
	"corpord-api/internal/config"
	"corpord-api/internal/database"
	"corpord-api/internal/handler"
	"corpord-api/internal/logger"
	"corpord-api/internal/repository"
	"corpord-api/internal/server"
	"corpord-api/internal/service"
	"errors"
	"log"
	"time"
)

type App struct {
	cfg    *config.Config
	logger *logger.Logger
	r      *repository.Repository
	s      *service.Service
	h      handler.Handler
	srv    server.Server
	ctx    context.Context
	db     *database.Database
}

func New() *App {
	a := &App{}

	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("couldn't load config: %v", err)
	}
	a.cfg = cfg

	a.logger, err = logger.New(&cfg.Logger)
	if err != nil {
		log.Fatalf("couldn't initialize logger: %v", err)
	}
	defer func() {
		if err := a.logger.Sync(); err != nil {
			log.Printf("failed to sync logger: %v", err)
		}
	}()

	a.logger.Info("initializing application")

	a.db = database.New(context.TODO(), &a.cfg.Database)
	if a.cfg.Env == "development" {
		a.logger.Info("applying database migrations")
		if err := a.db.Postgres.MigrateUp(context.TODO()); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.logger.Fatal("migration timed out. Please check if the database is accessible and try again")
			}
			a.logger.Fatalf("failed to apply migrations: %v", err)
		}
	}

	a.logger.Info("initializing repository layer")
	a.r = repository.New(a.logger, a.db.Postgres.DB())

	a.logger.Info("initializing service layer")
	a.s = service.New(a.logger, a.r)

	a.logger.Info("initializing handler layer")
	a.h = handler.New(a.logger, a.s)

	a.logger.Info("initializing server")
	a.srv = server.New(a.h.InitRoutes())

	a.logger.Info("application initialized successfully")
	return a
}

func (a *App) Start() error {
	a.logger.Info("starting application")
	return a.srv.Start()
}

func (a *App) Stop() {
	a.logger.Info("shutting down application")
	if err := a.logger.Sync(); err != nil {
		log.Printf("failed to sync logger: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.srv.Shutdown(ctx)
}
