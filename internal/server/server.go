package server

import (
	"context"
	"corpord-api/internal/config"
	"errors"
	"net/http"
	"strconv"
)

type Server interface {
	Start() error
	Shutdown(ctx context.Context) error
}

type server struct {
	srv *http.Server
}

func New(h http.Handler, cfg *config.HTTP) Server {
	return &server{
		srv: &http.Server{
			Addr:              ":" + strconv.Itoa(cfg.Port),
			Handler:           h,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

func (s *server) Start() error {
	return s.srv.ListenAndServe()
}

func (s *server) Shutdown(ctx context.Context) error {
	if err := s.srv.Shutdown(ctx); err != nil {
		return errors.Join(err, s.srv.Close())
	}
	return nil
}
