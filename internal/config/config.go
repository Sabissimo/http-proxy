// Package config loads the proxy's configuration.
//
// Values come from a .env file next to the executable (the Windows-service
// deployment — a service's working directory is System32, not the install
// folder), falling back to .env in the working directory. Real environment
// variables take precedence over the file.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Environment keys. PORT, TARGET, SECURE, PROXY_TIMEOUT_MS and LOGGING keep
// the names of the Node.js version, so an existing .env carries over.
const (
	KeyPort           = "PORT"
	KeyTarget         = "TARGET"
	KeySecure         = "SECURE"
	KeyProxyTimeoutMS = "PROXY_TIMEOUT_MS"
	KeyLogRequests    = "LOGGING"
	KeyHealthPath     = "HEALTH_PATH"

	KeyServiceName        = "SERVICE_NAME"
	KeyServiceDisplayName = "SERVICE_DISPLAY_NAME"
	KeyServiceDescription = "SERVICE_DESCRIPTION"

	KeyLogDir      = "LOG_DIR"
	KeyLogKeepDays = "LOG_KEEP_DAYS"
)

const (
	envFileName           = ".env"
	defaultPort           = 7777
	defaultProxyTimeoutMS = 60000
	defaultHealthPath     = "/healthz"
	healthPathOff         = "off"
	defaultLogKeepDays    = 30
	maxPort               = 65535
)

// Config is the proxy configuration.
type Config struct {
	Port         int
	Target       *url.URL      // upstream every request is forwarded to
	Secure       bool          // verify the upstream's TLS certificate
	ProxyTimeout time.Duration // how long to wait for the upstream's response headers
	LogRequests  bool          // log one line per proxied request
	HealthPath   string        // answered locally with 200; "" = forwarded like any other path
}

// Load reads and validates the configuration.
func Load() (*Config, error) {
	v, err := newViper()
	if err != nil {
		return nil, err
	}
	v.SetDefault(KeyPort, defaultPort)
	v.SetDefault(KeyProxyTimeoutMS, defaultProxyTimeoutMS)
	v.SetDefault(KeyHealthPath, defaultHealthPath)

	target, err := parseTarget(v.GetString(KeyTarget))
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Port:         v.GetInt(KeyPort),
		Target:       target,
		Secure:       v.GetBool(KeySecure),
		ProxyTimeout: time.Duration(v.GetInt(KeyProxyTimeoutMS)) * time.Millisecond,
		LogRequests:  v.GetBool(KeyLogRequests),
		HealthPath:   healthPath(v.GetString(KeyHealthPath)),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Port <= 0 || c.Port > maxPort {
		return fmt.Errorf("%s must be a TCP port, got %d", KeyPort, c.Port)
	}
	if c.ProxyTimeout < 0 {
		return fmt.Errorf("%s must be 0 (no limit) or a number of milliseconds", KeyProxyTimeoutMS)
	}
	if c.HealthPath != "" && !strings.HasPrefix(c.HealthPath, "/") {
		return fmt.Errorf("%s must start with / (or be %q to forward it), got %q", KeyHealthPath, healthPathOff, c.HealthPath)
	}
	return nil
}

// healthPath maps HEALTH_PATH=off to "" (no local health check).
func healthPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.EqualFold(raw, healthPathOff) {
		return ""
	}
	return raw
}

// parseTarget checks TARGET is an absolute http(s) URL.
func parseTarget(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%s is required (e.g. https://new.example.com)", KeyTarget)
	}
	target, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", KeyTarget, err)
	}
	if (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute http(s) URL like https://new.example.com, got %q", KeyTarget, raw)
	}
	return target, nil
}

// ListenAddr is the address the HTTP server binds to.
func (c *Config) ListenAddr() string { return fmt.Sprintf(":%d", c.Port) }

// Service is how the Windows service is registered. Empty fields mean "use the
// built-in default"; the service commands fill them in.
type Service struct {
	Name        string
	DisplayName string
	Description string
}

// LoadService reads the service registration values. Unlike Load it needs no
// TARGET, so `service install` works before the rest of .env is filled in.
func LoadService() (Service, error) {
	v, err := newViper()
	if err != nil {
		return Service{}, err
	}
	return Service{
		Name:        strings.TrimSpace(v.GetString(KeyServiceName)),
		DisplayName: strings.TrimSpace(v.GetString(KeyServiceDisplayName)),
		Description: strings.TrimSpace(v.GetString(KeyServiceDescription)),
	}, nil
}

// Log is where the service writes its daily log files.
type Log struct {
	Dir      string // "" = logs next to the executable
	KeepDays int    // files older than this are deleted; 0 = keep all
}

// LoadLog reads LOG_DIR and LOG_KEEP_DAYS. Like LoadService it needs no
// TARGET, so logging is set up before it is checked.
func LoadLog() (Log, error) {
	v, err := newViper()
	if err != nil {
		return Log{}, err
	}
	v.SetDefault(KeyLogKeepDays, defaultLogKeepDays)
	out := Log{Dir: strings.TrimSpace(v.GetString(KeyLogDir)), KeepDays: v.GetInt(KeyLogKeepDays)}
	if out.KeepDays < 0 {
		return Log{}, fmt.Errorf("%s must be 0 (keep all) or a number of days", KeyLogKeepDays)
	}
	return out, nil
}

// newViper reads the environment, seeded from .env when there is one.
func newViper() (*viper.Viper, error) {
	v := viper.New()
	v.AutomaticEnv()
	if path := findEnvFile(); path != "" {
		v.SetConfigFile(path)
		v.SetConfigType("env")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
	}
	return v, nil
}

// findEnvFile returns the first existing .env: next to the executable, then in
// the working directory. Empty when there is none (environment only).
func findEnvFile() string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), envFileName))
	}
	candidates = append(candidates, envFileName)
	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
			return path
		}
	}
	return ""
}
