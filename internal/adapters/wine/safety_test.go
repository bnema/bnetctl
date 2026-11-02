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

func TestLockRetriesInterruptedInitialCreation(t *testing.T) {
	a, prefix, calls := migrationFixture(t)
	rt := a.detected
	// Provide deterministic detection while preserving the graphics fixture.
	mustWrite(t, rt.WineBin, "#!/bin/sh\nif [ \"$1\" = --version ]; then echo wine-11.21; exit 0; fi\nif [ \"$1\" = reg ] && [ \"$2\" = query ]; then echo \"$5 REG_SZ native\"; fi\n")
	mustWrite(t, rt.WineServerBin, "#!/bin/sh\nif [ \"$1\" = -k0 ]; then exit 1; fi\n")
	mustWrite(t, rt.WineBootBin, "#!/bin/sh\necho init >> '"+calls+"'\nprintf 'WINE REGISTRY Version 2\\n#arch=win64\\n' > \"$WINEPREFIX/system.reg\"\n")
	t.Setenv(wineOverrideEnv, rt.WineBin)
	// Detect discovers these setup paths; graphicsPlan uses their package data.
	tools := t.TempDir()
	for name, target := range map[string]string{"setup_dxvk": rt.DXVKSetupBin, "setup_vkd3d_proton": rt.VKD3DSetupBin} {
		if err := os.Symlink(target, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools)
	if err := os.Remove(filepath.Join(prefix, "pfx/system.reg")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-11.21\n")
	release, err := a.LockPrefix(prefix)
	if err != nil {
		t.Fatal(err)
	}
	release()
	version, err := readVersion(filepath.Join(prefix, prefixVersionFile))
	if err != nil || version != "wine-11.21" {
		t.Fatalf("version: %s %v", version, err)
	}
	if _, err := os.Stat(filepath.Join(prefix, prefixVersionFile+".migrating")); !os.IsNotExist(err) {
		t.Fatalf("pending record retained: %v", err)
	}
	data, err := os.ReadFile(calls)
	if err != nil || !strings.Contains(string(data), "init") {
		t.Fatalf("init not retried: %s %v", data, err)
	}
	// Missing or malformed architecture on an established prefix is not initial creation.
	for _, contents := range []string{"", "WINE REGISTRY Version 2\n#arch=garbage\n"} {
		if contents == "" {
			if err := os.Remove(filepath.Join(prefix, "pfx/system.reg")); err != nil {
				t.Fatal(err)
			}
		} else {
			mustWrite(t, filepath.Join(prefix, "pfx/system.reg"), contents)
		}
		mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-11.21\n")
		if unlock, err := a.LockPrefix(prefix); err == nil {
			unlock()
			t.Fatal("established prefix with invalid architecture accepted")
		}
	}
}

func TestInterruptedInitialCreationGuards(t *testing.T) {
	for _, guard := range []string{"running", "RunOnce", "boot failure", "malformed architecture"} {
		t.Run(guard, func(t *testing.T) {
			dir := fakeVersionInstall(t, "wine-11.21")
			t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
			prefix := t.TempDir()
			if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0755); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(prefix, prefixVersionFile+".migrating"), "wine-11.21\n")
			calls := filepath.Join(t.TempDir(), "boot-calls")
			mustWrite(t, filepath.Join(dir, "wineboot"), "#!/bin/sh\necho init >> '"+calls+"'\nexit 1\n")
			switch guard {
			case "running":
				mustWrite(t, filepath.Join(dir, "wineserver"), "#!/bin/sh\nexit 0\n")
			case "RunOnce":
				mustWrite(t, filepath.Join(prefix, "pfx/user.reg"), "[Software\\\\Microsoft\\\\Windows\\\\CurrentVersion\\\\RunOnce]\n\"pending\"=\"run\"\n")
			case "malformed architecture":
				mustWrite(t, filepath.Join(prefix, "pfx/system.reg"), "#arch=garbage\n")
			}
			a := NewAdapter(log.New(io.Discard))
			if unlock, err := a.LockPrefix(prefix); err == nil {
				unlock()
				t.Fatal("unsafe initialization accepted")
			}
			if _, err := os.Stat(filepath.Join(prefix, prefixVersionFile)); !os.IsNotExist(err) {
				t.Fatalf("committed version: %v", err)
			}
			if _, err := os.Stat(filepath.Join(prefix, prefixVersionFile+".migrating")); err != nil {
				t.Fatalf("lost retry marker: %v", err)
			}
			if _, err := os.Stat(calls); guard == "boot failure" {
				if err != nil {
					t.Fatal("boot retry did not run")
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("guard ran wineboot")
			}
			// Every failure releases exclusion for a later retry or shutdown.
			unlock, err := a.lockPrefixExclusion(prefix, 0)
			if err != nil {
				t.Fatal(err)
			}
			unlock()
		})
	}
}
