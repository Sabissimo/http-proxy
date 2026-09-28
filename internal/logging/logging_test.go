package logging

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	exeDir := filepath.FromSlash("C:/Apps/HttpProxy")
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{name: "default", dir: "", want: "C:/Apps/HttpProxy/logs"},
		{name: "absolute", dir: filepath.FromSlash("D:/Logs/HttpProxy"), want: "D:/Logs/HttpProxy"},
		{name: "relative to the exe", dir: "var/log", want: "C:/Apps/HttpProxy/var/log"},
		{name: "trailing separator", dir: filepath.FromSlash("D:/Logs/"), want: "D:/Logs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(exeDir, tt.dir); got != filepath.FromSlash(tt.want) {
				t.Errorf("got %q, want %q", got, filepath.FromSlash(tt.want))
			}
		})
	}
}

func TestDayOf(t *testing.T) {
	tests := []struct {
		name string
		day  string
		ok   bool
	}{
		{name: "httpproxy-2026-09-23.log", day: "2026-09-23", ok: true},
		{name: "httpproxy.log"},
		{name: "httpproxy-2026-13-01.log"},
		{name: "other-2026-09-23.log"},
		{name: "httpproxy-2026-09-23.log.bak"},
	}
	for _, tt := range tests {
		day, ok := dayOf(tt.name)
		if day != tt.day || ok != tt.ok {
			t.Errorf("dayOf(%q) = %q, %v; want %q, %v", tt.name, day, ok, tt.day, tt.ok)
		}
	}
}

// TestDailyFile writes across midnight and checks the switch and the pruning.
func TestDailyFile(t *testing.T) {
	dir := t.TempDir()
	for _, old := range []string{"httpproxy-2026-08-01.log", "httpproxy-2026-09-20.log", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, old), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 23, 23, 59, 0, 0, time.Local)
	w := newDailyFile(dir, 30)
	w.now = func() time.Time { return now }

	if _, err := w.Write([]byte("before midnight\n")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := w.Write([]byte("after midnight\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{
		"httpproxy-2026-09-20.log", // within 30 days: kept
		"httpproxy-2026-09-23.log",
		"httpproxy-2026-09-24.log",
		"notes.txt", // not a daily log: never touched
	}
	if !slices.Equal(names, want) {
		t.Errorf("files = %v, want %v", names, want)
	}
	got, err := os.ReadFile(filepath.Join(dir, "httpproxy-2026-09-24.log"))
	if err != nil || string(got) != "after midnight\n" {
		t.Errorf("new day file = %q, %v", got, err)
	}
}
