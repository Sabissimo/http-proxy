package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sabissimo/http-proxy/internal/config"
)

const testTimeout = 5 * time.Second

// newProxy starts the proxy in front of upstream and returns its URL.
func newProxy(t *testing.T, upstream string, modify func(*config.Config)) string {
	t.Helper()
	target, err := url.Parse(upstream)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Target: target, ProxyTimeout: testTimeout, HealthPath: "/healthz", LogRequests: true}
	if modify != nil {
		modify(cfg)
	}
	srv := httptest.NewServer(Handler(cfg))
	t.Cleanup(srv.Close)
	return srv.URL
}

// echoUpstream answers with what it received.
func echoUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.Header().Set("X-Upstream", "yes")
		fmt.Fprintf(w, "%s %s host=%s xff=%s body=%s",
			r.Method, r.URL.RequestURI(), r.Host, r.Header.Get("X-Forwarded-For"), body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHandler(t *testing.T) {
	upstream := echoUpstream(t)
	upstreamHost := strings.TrimPrefix(upstream.URL, "http://")
	tests := []struct {
		name       string
		target     string
		modify     func(*config.Config)
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:   "forwards method, path, query, body; Host is the target's",
			method: http.MethodPost, path: "/api/items?id=7&x=a%20b", body: "hello",
			wantStatus: http.StatusOK,
			wantBody:   "POST /api/items?id=7&x=a%20b host=" + upstreamHost + " xff=127.0.0.1 body=hello",
		},
		{
			name: "target path is prefixed", target: upstream.URL + "/base",
			method: http.MethodGet, path: "/x",
			wantStatus: http.StatusOK, wantBody: "GET /base/x host=" + upstreamHost + " xff=127.0.0.1 body=",
		},
		{
			name: "health answered locally", method: http.MethodGet, path: "/healthz",
			wantStatus: http.StatusOK, wantBody: healthBody,
		},
		{
			name: "health path is forwarded for other methods", method: http.MethodPost, path: "/healthz",
			wantStatus: http.StatusOK, wantBody: "POST /healthz host=" + upstreamHost + " xff=127.0.0.1 body=",
		},
		{
			name: "health check off: forwarded", modify: func(c *config.Config) { c.HealthPath = "" },
			method: http.MethodGet, path: "/healthz",
			wantStatus: http.StatusOK, wantBody: "GET /healthz host=" + upstreamHost + " xff=127.0.0.1 body=",
		},
		{
			name: "upstream down: 502", target: "http://127.0.0.1:1",
			method: http.MethodGet, path: "/",
			wantStatus: http.StatusBadGateway, wantBody: "Bad gateway",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			if target == "" {
				target = upstream.URL
			}
			proxyURL := newProxy(t, target, tt.modify)
			req, err := http.NewRequest(tt.method, proxyURL+tt.path, strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.wantStatus || string(got) != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", resp.StatusCode, got, tt.wantStatus, tt.wantBody)
			}
		})
	}
}

func TestUpstreamTimeout(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { close(release) })

	proxyURL := newProxy(t, upstream.URL, func(c *config.Config) { c.ProxyTimeout = 50 * time.Millisecond })
	resp, err := http.Get(proxyURL + "/slow")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
}

// TestStreaming checks a chunk reaches the client while the upstream is still
// writing — nothing is buffered until the response ends.
func TestStreaming(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "first\n")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "second\n")
	}))
	t.Cleanup(upstream.Close)

	resp, err := http.Get(newProxy(t, upstream.URL, nil) + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := make(chan string)
	go func() {
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		lines <- line
	}()
	select {
	case line := <-lines:
		if line != "first\n" {
			t.Errorf("first line = %q", line)
		}
	case <-time.After(testTimeout):
		t.Fatal("first chunk was held back")
	}
	close(release)
}

// TestWebSocket upgrades through the proxy and echoes over the raw tunnel.
func TestWebSocket(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "not an upgrade", http.StatusBadRequest)
			return
		}
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprint(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		buf.Flush()
		line, _ := buf.ReadString('\n')
		fmt.Fprint(buf, "echo: "+line)
		buf.Flush()
	}))
	t.Cleanup(upstream.Close)

	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(newProxy(t, upstream.URL, nil), "http://"), testTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(testTimeout)); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(conn, "GET /ws HTTP/1.1\r\nHost: proxy\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}
	fmt.Fprint(conn, "ping\n")
	if line, err := reader.ReadString('\n'); err != nil || line != "echo: ping\n" {
		t.Errorf("got %q, %v; want echo", line, err)
	}
}
