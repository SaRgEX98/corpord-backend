package main

import (
	"context"
	"corpord-api/internal/app"
	"log"
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

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func main() {
	sig, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	a := *app.New()
	go func() {
		if err := a.Start(); err != nil {
			log.Fatal("couldn't start app")
		}
	}()
	<-sig.Done()
}
