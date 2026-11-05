package wine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	wineLogCountLimit        = 3
	wineLogTotalBudget int64 = 100 << 20
	wineLogFileLimit   int64 = 50 << 20
)

// launchStderr uses a file descriptor directly: Wine descendants can retain it
// without a copy goroutine delaying cmd.Wait until every descendant exits.
func (a *Adapter) launchStderr() *os.File {
	file, err := openWineLog()
	if err != nil {
		a.log.Warn("cannot create Wine log; using terminal stderr", "error", err)
		return os.Stderr
	}
	a.log.Info("Wine stderr log", "path", file.Name())
	if debug, set := os.LookupEnv("BNETCTL_WINEDEBUG"); set && debug != "err+all,warn+waylanddrv" {
		a.log.Warn("custom WINEDEBUG can produce large logs", "path", file.Name())
	}
	return file
}

func openWineLog() (*os.File, error) {
	state := os.Getenv("XDG_STATE_HOME")
	if state == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		state = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(state, "bnetctl", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Fixed-width UTC timestamps sort chronologically; the random suffix avoids
	// collisions when launches occur together.
	file, err := os.CreateTemp(dir, "wine-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-*.log")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*os.File, error) {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	if err := rotateWineLogs(dir, filepath.Base(file.Name())); err != nil {
		return fail(err)
	}
	// Build the replacement beside latest.log, then rename it atomically.
	link := file.Name() + ".latest"
	if err := os.Symlink(filepath.Base(file.Name()), link); err != nil {
		return fail(err)
	}
	if err := os.Rename(link, filepath.Join(dir, "latest.log")); err != nil {
		_ = os.Remove(link)
		return fail(err)
	}
	return file, nil
}

// rotateWineLogs always reserves one slot for the new log, regardless of its
// name or size. Only old files are subject to size limits at launch time.
func rotateWineLogs(dir, newName string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.Name() != newName && entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "wine-") && strings.HasSuffix(entry.Name(), ".log") {
			names = append(names, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var kept []string
	for i, name := range names {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if i >= wineLogCountLimit-1 || info.Size() > wineLogFileLimit {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("rotate Wine logs: %w", err)
			}
		} else {
			kept = append(kept, name)
		}
	}
	return pruneWineLogBudget(dir, kept)
}

// names are newest first. Remove oldest files until the old-log budget fits.
func pruneWineLogBudget(dir string, names []string) error {
	sizes := make([]int64, len(names))
	var total int64
	for i, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		sizes[i] = info.Size()
		total += sizes[i]
	}
	for i := len(names) - 1; i >= 0 && total > wineLogTotalBudget; i-- {
		if err := os.Remove(filepath.Join(dir, names[i])); err != nil {
			return fmt.Errorf("rotate Wine logs: %w", err)
		}
		total -= sizes[i]
	}
	return nil
}
