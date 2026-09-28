// Package server runs the HTTP listener: the local health check, and the
// reverse proxy for everything else.
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Sabissimo/http-proxy/internal/config"
	"github.com/Sabissimo/http-proxy/internal/proxy"
)

const (
	readHeaderTimeout = 30 * time.Second
	shutdownTimeout   = 10 * time.Second
	healthBody        = "ok"
)

// Handler is the proxy for cfg, with cfg.HealthPath answered locally so a
// deploy or monitor can tell the proxy is up without depending on the
// upstream.
func Handler(cfg *config.Config) http.Handler {
	next := proxy.New(cfg)
	if cfg.HealthPath == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == cfg.HealthPath && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			writeHealth(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.WriteString(w, healthBody); err != nil {
		slog.Debug("write health body", "err", err)
	}
}

// Serve listens on cfg.Port until ctx is cancelled, then lets in-flight
// requests finish (up to shutdownTimeout).
func Serve(ctx context.Context, cfg *config.Config) error {
	listener, err := net.Listen("tcp", cfg.ListenAddr())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddr(), err)
	}
	srv := &http.Server{Handler: Handler(cfg), ReadHeaderTimeout: readHeaderTimeout}
	slog.Info("reverse proxy listening",
		"addr", cfg.ListenAddr(), "target", cfg.Target.String(), "secure", cfg.Secure,
		"health_path", cfg.HealthPath, "log_requests", cfg.LogRequests)

	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	select {
	case err := <-served:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	return shutdown(srv, served)
}

func shutdown(srv *http.Server, served <-chan error) error {
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	slog.Info("stopped")
	return nil
}
