package server

import (
	"context"
	"corpord-api/internal/config"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServerConfigAndGracefulShutdown(t *testing.T) {
	cfg := &config.HTTP{Port: 9123, ReadHeaderTimeout: time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 4 * time.Second}
	started, release := make(chan struct{}), make(chan struct{})
	s := New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; w.WriteHeader(204) }), cfg).(*server)
	if s.srv.Addr != ":9123" || s.srv.ReadHeaderTimeout != cfg.ReadHeaderTimeout || s.srv.WriteTimeout != cfg.WriteTimeout || s.srv.ReadTimeout != cfg.ReadTimeout || s.srv.IdleTimeout != cfg.IdleTimeout {
		t.Fatal("HTTP settings ignored")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.srv.Close() })
	serving := make(chan error, 1)
	go func() { serving <- s.srv.Serve(listener) }()
	requestDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode != 204 {
				err = errors.New("request did not finish")
			}
		}
		requestDone <- err
	}()
	<-started
	shutdown := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdown <- s.Shutdown(ctx)
	}()
	close(release)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	if err := <-shutdown; err != nil {
		t.Fatal(err)
	}
	if err := <-serving; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("serve error: %v", err)
	}
}
