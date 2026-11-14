package server

import (
	"context"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

type Server struct {
	http *http.Server
}

func New(addr string, handler http.Handler) *Server {
	return &Server{http: &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}}
}

func (s *Server) Start() error {
	log.Infof("Starting server on %s", s.http.Addr)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
