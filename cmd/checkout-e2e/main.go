//go:build checkout_e2e

package main

import (
	"corpord-api/internal/app"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	a := app.NewCheckoutE2E()
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-stopped; a.Stop() }()
	if err := a.Start(); err != nil {
		log.Print("test backend stopped")
	}
}
