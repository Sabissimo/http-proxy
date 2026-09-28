package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "defaults",
			env:  map[string]string{KeyTarget: "https://new.example.com"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Port != defaultPort || cfg.Secure || cfg.LogRequests ||
					cfg.ProxyTimeout != time.Minute || cfg.HealthPath != defaultHealthPath {
					t.Errorf("unexpected defaults: %+v", cfg)
				}
				if cfg.Target.Host != "new.example.com" {
					t.Errorf("target host = %q", cfg.Target.Host)
				}
			},
		},
		{
			name: "everything set",
			env: map[string]string{
				KeyTarget: "http://10.0.0.5:8080/app", KeyPort: "8081", KeySecure: "true",
				KeyProxyTimeoutMS: "1500", KeyLogRequests: "TRUE", KeyHealthPath: "/_proxy/health",
			},
			check: func(t *testing.T, cfg *Config) {
				if cfg.Port != 8081 || !cfg.Secure || !cfg.LogRequests ||
					cfg.ProxyTimeout != 1500*time.Millisecond || cfg.HealthPath != "/_proxy/health" {
					t.Errorf("unexpected config: %+v", cfg)
				}
			},
		},
		{
			name: "health check off",
			env:  map[string]string{KeyTarget: "https://x.example.com", KeyHealthPath: "Off"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.HealthPath != "" {
					t.Errorf("health path = %q, want empty", cfg.HealthPath)
				}
			},
		},
		{name: "no target", env: map[string]string{}, wantErr: true},
		{name: "relative target", env: map[string]string{KeyTarget: "new.example.com"}, wantErr: true},
		{name: "ftp target", env: map[string]string{KeyTarget: "ftp://new.example.com"}, wantErr: true},
		{name: "port out of range", env: map[string]string{KeyTarget: "https://x.example.com", KeyPort: "70000"}, wantErr: true},
		{name: "negative timeout", env: map[string]string{KeyTarget: "https://x.example.com", KeyProxyTimeoutMS: "-1"}, wantErr: true},
		{name: "health path without slash", env: map[string]string{KeyTarget: "https://x.example.com", KeyHealthPath: "healthz"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{KeyTarget, KeyPort, KeySecure, KeyProxyTimeoutMS, KeyLogRequests, KeyHealthPath} {
				t.Setenv(key, tt.env[key])
			}
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadLog(t *testing.T) {
	tests := []struct {
		name     string
		keepDays string
		want     int
		wantErr  bool
	}{
		{name: "default", want: defaultLogKeepDays},
		{name: "keep all", keepDays: "0", want: 0},
		{name: "negative", keepDays: "-3", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(KeyLogKeepDays, tt.keepDays)
			got, err := LoadLog()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.KeepDays != tt.want {
				t.Errorf("keep days = %d, want %d", got.KeepDays, tt.want)
			}
		})
	}
}
