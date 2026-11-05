package app

import (
	"context"
	"corpord-api/internal/booking"
	"corpord-api/internal/checkout"
	"corpord-api/internal/config"
	"corpord-api/internal/database"
	"corpord-api/internal/handler"
	"corpord-api/internal/logger"
	"corpord-api/internal/payments"
	"corpord-api/internal/repository"
	"corpord-api/internal/scheduler"
	"corpord-api/internal/server"
	"corpord-api/internal/service"
	"corpord-api/internal/sso"
	"corpord-api/internal/token"
	"corpord-api/pkg/dbx"
	"errors"
	"github.com/gin-gonic/gin"
	"log"
	"syscall"
	"time"
)

type App struct {
	cfg               *config.Config
	logger            *logger.Logger
	r                 *repository.Repository
	s                 *service.Service
	h                 handler.Handler
	srv               server.Server
	ctx               context.Context
	db                *database.Database
	t                 token.Manager
	qb                *dbx.QueryBuilder
	sso               *sso.Registry
	scheduler         *scheduler.Scheduler
	bookingScheduler  *scheduler.Scheduler
	checkoutScheduler *scheduler.Scheduler
}

func New() *App { return newApp(nil, nil) }

// Provider/router injection is used by the build-tagged disposable E2E binary.
// Production New always supplies nil and cannot enable a fake provider by env.
func newApp(testProvider payments.Provider, configure func(*gin.Engine)) *App {
	a := &App{}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("couldn't load config: %v", err)
	}
	if testProvider != nil {
		if cfg.App.Env != "development" {
			log.Fatal("test provider requires development environment")
		}
		cfg.Payments.Enabled = true
		cfg.Payments.ReturnOrigin = cfg.FrontendURL
	}
	a.cfg = cfg

	a.logger, err = logger.New(&cfg.Logger)
	if err != nil {
		log.Fatalf("couldn't initialize logger: %v", err)
	}

	a.logger.Info("initializing application")

	a.db = database.New(context.TODO(), &a.cfg.Database)
	if a.cfg.App.Env == "development" {
		a.logger.Info("applying database migrations")
		if err := a.db.Postgres.MigrateUp(context.TODO()); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.logger.Fatal("migration timed out. Please check if the database is accessible and try again")
			}
			a.logger.Fatalf("failed to apply migrations: %v", err)
		}
	}

	a.qb = dbx.NewQueryBuilder(a.db.Postgres.DB())

	a.logger.Info("initializing repository layer")
	a.r = repository.New(a.logger, a.qb)

	a.logger.Info("initializing token manager")
	a.t = token.NewManager(&a.cfg.JWT)

	a.logger.Info("initializing sso registry")
	a.sso = sso.NewRegistry()

	if a.cfg.SSO.Google.Enabled {
		google := sso.NewGoogleProvider(
			a.cfg.SSO.Google.ClientID,
			a.cfg.SSO.Google.ClientSecret,
			a.cfg.SSO.Google.RedirectURL,
		)
		a.sso.Register("google", google)
	}

	if a.cfg.SSO.Yandex.Enabled {
		yandex := sso.NewYandexProvider(
			a.cfg.SSO.Yandex.ClientID,
			a.cfg.SSO.Yandex.ClientSecret,
			a.cfg.SSO.Yandex.RedirectURL,
		)
		a.sso.Register("yandex", yandex)
	}

	a.logger.Info("initializing service layer")
	a.s = service.New(a.logger, a.r, a.t, a.sso)
	var provider payments.Provider = testProvider
	if cfg.Payments.Enabled && provider == nil {
		client, e := payments.NewYooKassa(payments.Config{ShopID: cfg.Payments.ShopID, SecretKey: cfg.Payments.SecretKey, Timeout: cfg.Payments.Timeout})
		if e != nil {
			log.Fatal("invalid test payment provider configuration")
		}
		provider = client
	}
	a.s.Checkout = checkout.New(a.db.Postgres.DB(), provider, checkout.Options{Enabled: cfg.Payments.Enabled, ReturnOrigin: cfg.Payments.ReturnOrigin})

	a.logger.Info("initializing handler layer")
	if a.cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	a.h = handler.New(a.logger, a.s, a.cfg, a.t, a.sso,
		handler.WithSSOStateStore(sso.NewRedisStateStore(a.db.Redis.Client())),
		handler.WithSSOFrontendURL(cfg.FrontendURL))

	a.logger.Info("initializing server")
	engine := a.h.InitRoutes()
	if configure != nil {
		configure(engine)
	}
	a.srv = server.New(engine, &cfg.HTTP)

	a.scheduler = scheduler.New(a.logger, time.Hour)

	// Добавляем задачу очистки токенов
	cleanupTask := scheduler.NewCleanupRefreshTokensTask(a.r.PgRepository.RefreshToken, a.logger)
	a.scheduler.AddTask(cleanupTask)

	// Запускаем планировщик
	a.scheduler.Start()
	a.bookingScheduler = scheduler.New(a.logger, time.Minute)
	a.bookingScheduler.AddTask(scheduler.NewBookingExpiryTask(booking.New(a.db.Postgres.DB())))
	a.bookingScheduler.Start()
	a.checkoutScheduler = scheduler.NewWithTaskTimeout(a.logger, 2*time.Second, 10*time.Second)
	a.checkoutScheduler.AddTask(scheduler.NewCheckoutTask(a.s.Checkout))
	a.checkoutScheduler.Start()

	a.logger.Info("application initialized successfully")
	return a
}

func (a *App) Start() error {
	a.logger.Info("starting application")
	return a.srv.Start()
}

func (a *App) Stop() {
	a.logger.Info("shutting down application")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if a.scheduler != nil {
		a.scheduler.Stop()
	}
	if a.bookingScheduler != nil {
		a.bookingScheduler.Stop()
	}

	if a.checkoutScheduler != nil {
		a.checkoutScheduler.Stop()
	}
	if err := a.srv.Shutdown(ctx); err != nil {
		a.logger.Errorf("HTTP shutdown: %v", err)
	}
	if a.db != nil {
		a.db.Close()
	}
	if err := a.logger.Sync(); err != nil && !errors.Is(err, syscall.ENOTSUP) && !errors.Is(err, syscall.EINVAL) {
		log.Printf("failed to sync logger: %v", err)
	}
}
