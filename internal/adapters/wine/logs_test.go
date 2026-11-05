package wine

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/charmbracelet/log"
)

func TestWineLogRotationAndLatest(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	dir := filepath.Join(state, "bnetctl", "logs")
	var paths []string
	for i := 0; i < 8; i++ {
		file, err := openWineLog()
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, file.Name())
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		target, err := os.Readlink(filepath.Join(dir, "latest.log"))
		if err != nil || target != filepath.Base(file.Name()) {
			t.Fatalf("latest = %q, %v", target, err)
		}
	}
	logs, err := filepath.Glob(filepath.Join(dir, "wine-*.log"))
	if err != nil || len(logs) != 3 {
		t.Fatalf("logs = %v, %v", logs, err)
	}
	for i, path := range paths {
		_, err := os.Stat(path)
		if i < 5 && !os.IsNotExist(err) {
			t.Fatalf("old log retained: %s (%v)", path, err)
		}
		if i >= 5 && err != nil {
			t.Fatalf("recent log missing: %s (%v)", path, err)
		}
	}
}

func TestWineLogHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	file, err := openWineLog()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	if filepath.Dir(file.Name()) != filepath.Join(home, ".local", "state", "bnetctl", "logs") {
		t.Fatal(file.Name())
	}
}

func TestLaunchStderr(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "blocking", true: "async"}[async], func(t *testing.T) {
			state := t.TempDir()
			t.Setenv("XDG_STATE_HOME", state)
			script := filepath.Join(t.TempDir(), "wine")
			if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'driver warning\\n' >&2\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			adapter := NewAdapter(log.New(io.Discard))
			adapter.detected = &domain.WineRuntime{WineBin: script}
			if async {
				done, err := adapter.RunExeAsync(t.TempDir(), "Battle.net.exe", nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			} else if err := adapter.RunExe(t.TempDir(), "Battle.net.exe", nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(state, "bnetctl", "logs", "latest.log"))
			if err != nil || string(data) != "driver warning\n" {
				t.Fatalf("stderr = %q, %v", data, err)
			}
		})
	}
}

func TestWineLogFailureFallsBack(t *testing.T) {
	state := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(state, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", state)
	adapter := NewAdapter(log.New(io.Discard))
	if got := adapter.launchStderr(); got != os.Stderr {
		if err := got.Close(); err != nil {
			t.Error(err)
		}
		t.Fatal("expected terminal fallback")
	}
}

func sparseWineLog(t *testing.T, dir, name string, size int64) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}

func TestWineLogBudgetDeletesOldest(t *testing.T) {
	dir := t.TempDir()
	names := []string{"wine-3.log", "wine-2.log", "wine-1.log"}
	for _, name := range names {
		sparseWineLog(t, dir, name, 50<<20)
	}
	if err := pruneWineLogBudget(dir, names); err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		_, err := os.Stat(filepath.Join(dir, name))
		if i == 2 {
			if !os.IsNotExist(err) {
				t.Fatalf("oldest retained: %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWineLogOversizedAndNewExemption(t *testing.T) {
	dir := t.TempDir()
	// The new file sorts oldest and exceeds both limits, but must survive.
	sparseWineLog(t, dir, "wine-0.log", 200<<20)
	sparseWineLog(t, dir, "wine-3.log", (50<<20)+1)
	sparseWineLog(t, dir, "wine-2.log", 50<<20)
	sparseWineLog(t, dir, "wine-1.log", 1)
	if err := rotateWineLogs(dir, "wine-0.log"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wine-0.log", "wine-2.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"wine-3.log", "wine-1.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("log not removed: %s: %v", name, err)
		}
	}
}

func TestCustomWineDebugWarning(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		set, warn   bool
	}{
		{name: "unset"},
		{name: "default", set: true, value: "err+all,warn+waylanddrv"},
		{name: "disabled", set: true, value: "-all", warn: true},
		{name: "empty", set: true, warn: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("BNETCTL_WINEDEBUG", tt.value)
			if !tt.set {
				if err := os.Unsetenv("BNETCTL_WINEDEBUG"); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			adapter := NewAdapter(log.New(&output))
			f := adapter.launchStderr()
			t.Cleanup(func() {
				if err := f.Close(); err != nil {
					t.Error(err)
				}
			})
			count := strings.Count(output.String(), "custom WINEDEBUG can produce large logs")
			if (tt.warn && count != 1) || (!tt.warn && count != 0) {
				t.Fatal(output.String())
			}
			if tt.warn && !strings.Contains(output.String(), f.Name()) {
				t.Fatal("warning missing path")
			}
		})
	}
}
