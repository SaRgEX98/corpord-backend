package main

import (
	"context"
	"corpord-api/internal/app"
	"fmt"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	fmt.Print("hello")
	sig, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()

	a := *app.New()
	go func() {
		if err := a.Start(); err != nil {
			log.Fatal("couldn't start app")
		}
	}()
	<-sig.Done()
	log.Println("sadsg")
}
