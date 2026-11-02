package wine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"
)

func TestRunOncePreflight(t *testing.T) {
	for _, file := range []string{"system.reg", "user.reg"} {
		for _, key := range []string{
			`Software\\Microsoft\\Windows\\CurrentVersion\\RunOnce`,
			`Software\\Microsoft\\Windows\\CurrentVersion\\RunServicesOnce`,
			`Software\\Wow6432Node\\Microsoft\\Windows\\CurrentVersion\\RunOnce`,
			`Software\\Wow6432Node\\Microsoft\\Windows\\CurrentVersion\\RunServicesOnce`,
		} {
			t.Run(file+key, func(t *testing.T) {
				a, prefix, calls := migrationFixture(t)
				mustWrite(t, filepath.Join(prefix, "pfx", file), "["+key+"] 123\n\"Battle.net\"=\"run me\"\n")
				err := a.UpdatePrefix(prefix, nil)
				if err == nil || !strings.Contains(err.Error(), "Battle.net") || !strings.Contains(err.Error(), file) {
					t.Fatalf("preflight: %v", err)
				}
				if _, err := os.Stat(calls); !os.IsNotExist(err) {
					t.Fatal("preflight ran Wine")
				}
				if _, err := os.Stat(filepath.Join(prefix, prefixVersionFile+".migrating")); !os.IsNotExist(err) {
					t.Fatal("preflight mutated marker")
				}
				// An empty RunOnce key and populated Run key are safe with -r.
				mustWrite(t, filepath.Join(prefix, "pfx", file), "["+key+"] 123\n#time=abc\n[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\Run]\n\"Battle.net\"=\"run me\"\n")
				if err := checkRunOnce(filepath.Join(prefix, "pfx")); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func fakeVersionInstall(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"wine", "wineboot", "wineserver"} {
		mustWrite(t, filepath.Join(dir, name), "#!/bin/sh\nif [ \"$1\" = '--version' ]; then echo '"+version+"'; exit 0; fi\nexit 97\n")
	}
	return dir
}
func TestPrefixLockAndMinimum(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	prefix := t.TempDir()
	mustWrite(t, filepath.Join(prefix, prefixVersionFile), "wine-11.19\n")
	a := NewAdapter(log.New(io.Discard))
	release, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	b := NewAdapter(log.New(io.Discard))
	if _, err := b.LockPrefix(prefix); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("second lock: %v", err)
	}
	release()
	release()
	release2, err := b.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	release2()
	// The in-progress version is also a floor, even with an older committed marker.
	mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-12.0\n")
	if _, err := a.LockPrefix(prefix); err == nil || !strings.Contains(err.Error(), "refusing Wine downgrade") {
		t.Fatalf("recovery downgrade: %v", err)
	}
}
func TestMinimumWineSelection(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv("PATH", dir)
	t.Setenv(wineOverrideEnv, "")
	path, version, err := chooseMinimumWine("old", "wine-10.0", "wine-11.21")
	if err != nil || path != filepath.Join(dir, "wine") || version != "wine-11.21" {
		t.Fatalf("selection: %s %s %v", path, version, err)
	}
	t.Setenv(wineOverrideEnv, "old")
	if _, _, err := chooseMinimumWine("old", "wine-10.0", "wine-11.21"); err == nil {
		t.Fatal("explicit downgrade accepted")
	}
	t.Setenv(wineOverrideEnv, "")
	if _, _, err := chooseMinimumWine("old", "wine-10.0", "wine-999.0"); err == nil {
		t.Fatal("unavailable minimum accepted")
	}
}
func TestVersionComparison(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want int
	}{{"wine-11.21", "wine-11.19", 1}, {"wine-10.0-20260425 (CachyOS)", "wine-11.21", -1}, {"wine-11.21.1", "wine-11.21", 1}, {"wine-11.21 (Staging)", "wine-11.21.0", 0}} {
		cmp, err := compareVersions(tt.a, tt.b)
		if err != nil || cmp != tt.want {
			t.Fatalf("%s vs %s: %d %v", tt.a, tt.b, cmp, err)
		}
	}
}
func TestUpdateRefusesDowngradeWithoutWine(t *testing.T) {
	a, prefix, calls := migrationFixture(t)
	mustWrite(t, filepath.Join(prefix, prefixVersionFile), "wine-12.0\n")
	if err := a.UpdatePrefix(prefix, nil); err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("result: %v", err)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("downgrade ran Wine")
	}
}

func TestLockPreflightDoesNotProbeWine(t *testing.T) {
	a, prefix, calls := migrationFixture(t)
	mustWrite(t, filepath.Join(prefix, "pfx", "user.reg"), "[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\RunOnce]\n@=\"pending\"\n")
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	if _, err := a.LockPrefix(prefix); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("preflight: %v", err)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatal("preflight probed Wine")
	}
}

func TestMatchingVersionAllowsRunOnce(t *testing.T) {
	a, prefix, _ := migrationFixture(t)
	mustWrite(t, filepath.Join(prefix, prefixVersionFile), "wine-11.21\n")
	mustWrite(t, filepath.Join(prefix, "pfx/user.reg"), "[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\RunOnce]\n\"pending\"=\"allowed on normal launch\"\n")
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	release, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := a.UpdatePrefix(prefix, nil); err != nil {
		t.Fatalf("matching version refused RunOnce: %v", err)
	}
	release()
	// Recovery still needs the preflight even when the committed version matches.
	mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-11.21\n")
	if _, err := a.LockPrefix(prefix); err == nil || !strings.Contains(err.Error(), "RunOnce") {
		t.Fatalf("recovery skipped preflight: %v", err)
	}
}

func TestShutdownSelectsRecordedMinimumWithoutPreflight(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	prefix := t.TempDir()
	mustWrite(t, filepath.Join(prefix, prefixVersionFile), "wine-11.19\n")
	mustWrite(t, filepath.Join(prefix, "pfx/user.reg"), "[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\RunOnce]\n\"pending\"=\"run\"\n")
	a := NewAdapter(log.New(io.Discard))
	if _, err := a.GracefulKillPrefix(prefix, 0); err != nil {
		t.Fatal(err)
	}
	if a.detected.Version != "wine-11.21" {
		t.Fatalf("runtime: %+v", a.detected)
	}
	mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-12.0\n")
	if _, err := a.GracefulKillPrefix(prefix, 0); err == nil || !strings.Contains(err.Error(), "downgrade") {
		t.Fatalf("downgrade: %v", err)
	}
}

func TestShutdownAndDeletionShareMaintenanceLock(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	prefix := filepath.Join(t.TempDir(), "prefix")
	a := NewAdapter(log.New(io.Discard))
	b := NewAdapter(log.New(io.Discard))
	release, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.GracefulKillPrefix(prefix, 0); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("stop during maintenance: %v", err)
	}
	called := false
	if err := b.WithStoppedPrefix(prefix, 0, func() error { called = true; return nil }); err == nil || called {
		t.Fatalf("delete during maintenance: called=%v err=%v", called, err)
	}
	release()
	// Pending migration actions must not prevent shutdown/deletion.
	if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(prefix, "pfx/user.reg"), "[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\RunOnce]\n\"pending\"=\"run\"\n")
	err = b.WithStoppedPrefix(prefix, 0, func() error {
		if err := os.RemoveAll(prefix); err != nil {
			return err
		}
		// Removal and recreation must not create a second lock inode.
		if err := os.MkdirAll(prefix, 0755); err != nil {
			return err
		}
		if unlock, err := a.LockPrefix(prefix); err == nil {
			unlock()
			t.Error("maintenance acquired lock during deletion")
		}
		if _, err := a.GracefulKillPrefix(prefix, 0); err == nil {
			t.Error("shutdown acquired lock during deletion")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}

func TestShutdownWaitsForMaintenance(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	prefix := filepath.Join(t.TempDir(), "prefix")
	a := NewAdapter(log.New(io.Discard))
	release, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		b := NewAdapter(log.New(io.Discard))
		_, err := b.GracefulKillPrefix(prefix, time.Second)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("shutdown did not wait: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not resume")
	}
}
