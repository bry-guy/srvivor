package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	addr    string
	log     *slog.Logger
	healthy atomic.Bool
	server  *http.Server
}

func New(addr string, logger *slog.Logger) *Server {
	if addr == "" {
		addr = ":8080"
	}
	return &Server{addr: addr, log: logger}
}

func (s *Server) SetHealthy(healthy bool) {
	s.healthy.Store(healthy)
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.Handle("/metrics", promhttp.Handler())

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.log.Warn("health server shutdown", "error", err)
		}
	}()

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("health server failed", "error", err)
		}
	}()
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.healthy.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"}); err != nil {
			s.log.Warn("write health response", "error", err)
		}
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		s.log.Warn("write health response", "error", err)
	}
}
