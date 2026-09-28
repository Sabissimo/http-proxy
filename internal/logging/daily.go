package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dailyFile is an io.Writer that writes to <dir>/<prefix>-YYYY-MM-DD.log,
// switching to a new file on the first write of each local day, and deletes
// files older than keepDays when it opens one (0 = keep everything).
type dailyFile struct {
	dir      string
	keepDays int
	now      func() time.Time

	mu   sync.Mutex
	day  string // date of the open file
	file *os.File
}

const (
	filePrefix = "httpproxy-"
	fileSuffix = ".log"
	dayLayout  = "2006-01-02"
)

func newDailyFile(dir string, keepDays int) *dailyFile {
	return &dailyFile{dir: dir, keepDays: keepDays, now: time.Now}
}

func (d *dailyFile) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if day := d.now().Format(dayLayout); day != d.day || d.file == nil {
		if err := d.open(day); err != nil {
			return 0, err
		}
	}
	return d.file.Write(p)
}

// open switches to the file for day and prunes old files.
func (d *dailyFile) open(day string) error {
	if d.file != nil {
		_ = d.file.Close()
		d.file = nil
	}
	f, err := os.OpenFile(d.path(day), os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFilePerm)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	d.file, d.day = f, day
	d.prune()
	return nil
}

func (d *dailyFile) path(day string) string {
	return filepath.Join(d.dir, filePrefix+day+fileSuffix)
}

// prune deletes daily log files older than keepDays. Only files matching the
// exact name pattern are touched. Failures are reported to stderr, since the
// logger itself is what is being set up.
func (d *dailyFile) prune() {
	if d.keepDays <= 0 {
		return
	}
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "log prune:", err)
		return
	}
	cutoff := d.now().AddDate(0, 0, -d.keepDays).Format(dayLayout)
	for _, e := range entries {
		day, ok := dayOf(e.Name())
		if !ok || e.IsDir() || day >= cutoff { // YYYY-MM-DD compares lexically
			continue
		}
		if err := os.Remove(filepath.Join(d.dir, e.Name())); err != nil {
			fmt.Fprintln(os.Stderr, "log prune:", err)
		}
	}
}

// dayOf returns the date part of a daily log file name.
func dayOf(name string) (string, bool) {
	day, ok := strings.CutPrefix(name, filePrefix)
	if !ok {
		return "", false
	}
	if day, ok = strings.CutSuffix(day, fileSuffix); !ok {
		return "", false
	}
	if _, err := time.Parse(dayLayout, day); err != nil {
		return "", false
	}
	return day, true
}

func (d *dailyFile) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.file == nil {
		return nil
	}
	err := d.file.Close()
	d.file = nil
	return err
}

// logDailyHint records where logs go, once, at startup.
func logDailyHint(d *dailyFile) {
	slog.Info("logging to daily files", "dir", d.dir, "keep_days", d.keepDays)
}
