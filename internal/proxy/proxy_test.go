package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusRecorder(t *testing.T) {
	tests := []struct {
		name   string
		handle func(w http.ResponseWriter)
		want   int
	}{
		{name: "nothing written", handle: func(http.ResponseWriter) {}, want: http.StatusOK},
		{name: "body only", handle: func(w http.ResponseWriter) { _, _ = w.Write([]byte("x")) }, want: http.StatusOK},
		{name: "explicit status", handle: func(w http.ResponseWriter) { w.WriteHeader(http.StatusTeapot) }, want: http.StatusTeapot},
		{
			name: "first status wins",
			handle: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusNotFound)
				w.WriteHeader(http.StatusOK)
			},
			want: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
			tt.handle(rec)
			if got := rec.Status(); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}
