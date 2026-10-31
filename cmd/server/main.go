package main

import (
	"context"
	"corpord-api/internal/app"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// @title           Ordering Bus API
// @version         1.0
// @description     This is a server for order private bus.
// @termsOfService  http://swagger.io/terms/

// @contact.name   Luchits Timofei
// @contact.url    http://example.com
// @contact.email  luchitstimofei@yandex.ru

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath  /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	sig, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(sig); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	a := app.New()
	stopped := make(chan error, 1)
	go func() { stopped <- a.Start() }()
	var err error
	select {
	case <-ctx.Done():
		a.Stop()
		err = <-stopped
	case err = <-stopped:
		a.Stop()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
