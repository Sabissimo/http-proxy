// Package logging sets up slog: text to stderr on a console, and to daily log
// files when running as a Windows service (a service has no console, so stderr
// would be lost). Files are <LOG_DIR>/httpproxy-YYYY-MM-DD.log, default
// LOG_DIR logs\ next to the executable; files older than LOG_KEEP_DAYS go.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	defaultDirName = "logs"
	logDirPerm     = 0o755
	logFilePerm    = 0o644
)

// Setup installs the default logger. dir is the configured log folder ("" =
// default); a relative dir is taken relative to the executable's folder, since
// a service's working directory is System32. keepDays <= 0 keeps every file.
// The returned closer releases the log file (a no-op on a console).
func Setup(interactive bool, dir string, keepDays int) (io.Closer, error) {
	if interactive {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		return io.NopCloser(nil), nil
	}
	folder, err := Dir(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(folder, logDirPerm); err != nil {
		return nil, fmt.Errorf("create log folder: %w", err)
	}
	w := newDailyFile(folder, keepDays)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, nil)))
	logDailyHint(w)
	return w, nil
}

// Dir resolves the configured log folder.
func Dir(dir string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return resolve(filepath.Dir(exe), dir), nil
}

func resolve(exeDir, dir string) string {
	switch {
	case dir == "":
		dir = filepath.Join(exeDir, defaultDirName)
	case !filepath.IsAbs(dir):
		dir = filepath.Join(exeDir, dir)
	}
	return filepath.Clean(dir)
}
