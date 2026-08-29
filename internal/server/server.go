package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
	startedAt  time.Time
}

func New(address string, source DataSource) *Server {
	result := &Server{startedAt: time.Now()}
	result.httpServer = &http.Server{
		Addr:              address,
		Handler:           result.Handler(source),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return result
}

func (s *Server) Address() string { return s.httpServer.Addr }

func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.httpServer.Addr, err)
	}
	return s.Serve(ctx, listener)
}

// Serve runs the HTTP server on an already-bound listener. Binding separately
// lets the tray lifecycle report Start failures synchronously.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-serveCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpServer.Shutdown(shutdownCtx)
	}()
	err := s.httpServer.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return fmt.Errorf("serve %s: %w", s.httpServer.Addr, err)
}
