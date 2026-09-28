// Package proxy forwards every request to the configured upstream, preserving
// method, headers, body and query, with WebSocket upgrades and streamed
// responses passed straight through.
//
// It is built on net/http/httputil.ReverseProxy rather than Fiber: fasthttp
// buffers bodies and cannot hijack a connection for a WebSocket tunnel, which
// is exactly what a transparent proxy needs.
package proxy

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/Sabissimo/http-proxy/internal/config"
)

const (
	// flushImmediately writes every chunk from the upstream as soon as it
	// arrives, like the Node.js version's piping (server-sent events, long
	// polling and downloads are never held back).
	flushImmediately = -1

	badGatewayBody = "Bad gateway"
)

// New returns the reverse-proxy handler for cfg.Target.
func New(cfg *config.Config) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(cfg.Target) // also sets Host to the target's (changeOrigin)
			r.SetXForwarded()    // X-Forwarded-For / -Host / -Proto
		},
		Transport:     newTransport(cfg),
		FlushInterval: flushImmediately,
		ErrorHandler:  handleError,
	}
	if !cfg.LogRequests {
		return rp
	}
	return logRequests(rp)
}

// newTransport is the default transport with the upstream's TLS check and the
// response-header timeout from the configuration. The timeout only bounds the
// wait for the upstream to start answering; a streamed body or an open
// WebSocket is never cut off by it.
func newTransport(cfg *config.Config) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// SECURE=false (the default, as in the Node.js version) accepts any
	// upstream certificate — for internal hosts with self-signed ones.
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: !cfg.Secure} //nolint:gosec // opt-in via SECURE
	transport.ResponseHeaderTimeout = cfg.ProxyTimeout
	return transport
}

// handleError answers 502 when the upstream cannot be reached or does not
// answer in time. A client that went away is not an upstream failure, so it is
// not logged.
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() == nil {
		slog.Warn("proxy error", "method", r.Method, "url", r.URL.RequestURI(), "err", err)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	if _, werr := io.WriteString(w, badGatewayBody); werr != nil {
		slog.Debug("write 502 body", "err", werr)
	}
}

// logRequests logs one line per request once it is done: method, URL,
// status, duration and client. A WebSocket is logged when it closes.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"url", r.URL.RequestURI(),
			"status", rec.Status(),
			"duration", time.Since(start).Round(time.Millisecond),
			"remote", r.RemoteAddr,
		)
	})
}

// statusRecorder remembers the status code written. Unwrap lets
// http.ResponseController (which ReverseProxy uses) still reach the
// underlying writer's Flush and Hijack, so streaming and WebSockets work.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(p []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(p)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Status is the code sent, 200 when the handler wrote nothing explicit.
func (s *statusRecorder) Status() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}
