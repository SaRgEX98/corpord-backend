package server

import (
	"context"
	"log"
	"net/http"
)

type Server interface {
	Start() error
	Shutdown(ctx context.Context)
}

type server struct {
	srv *http.Server
}

func New(h http.Handler) Server {
	return &server{
		srv: &http.Server{
			Addr:    ":8080",
			Handler: h,
		},
	}
}

func (s *server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *server) Shutdown(ctx context.Context) {
	err := s.srv.Shutdown(ctx)
	if err != nil {
		log.Fatal("Server error stop")
	}
}
