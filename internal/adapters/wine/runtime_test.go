package wine

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
)

func TestBuildEnvUnsetsInheritedVariables(t *testing.T) {
	t.Setenv("DISPLAY", ":1")
	adapter := &Adapter{}

	got := adapter.buildEnv("/tmp/pfx", &domain.WineEnv{Unset: []string{"DISPLAY"}})
	if _, exists := got["DISPLAY"]; exists {
		t.Fatal("DISPLAY remained in Wine environment")
	}
	if got["WINEPREFIX"] != "/tmp/pfx" {
		t.Fatalf("unexpected WINEPREFIX: %q", got["WINEPREFIX"])
	}
}

func TestDisableSystrayWritesExplorerRegistryKey(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "invocation.log")
	logFile = strings.ReplaceAll(logFile, "\\", "\\\\")

	wine := filepath.Join(dir, "wine")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo 'wine-10.0'; exit 0; fi\n" +
		"printf '%s\\n' \"$*\" >> '" + logFile + "'\n"
	if err := os.WriteFile(wine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wineboot", "wineserver"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(wineOverrideEnv, wine)

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.DisableSystray(t.TempDir()); err != nil {
		t.Fatalf("DisableSystray: %v", err)
	}

	got, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read invocation log: %v", err)
	}
	want := `reg add HKCU\Software\Wine\Explorer /v ShowSystray /t REG_DWORD /d 0 /f`
	if !strings.Contains(string(got), want) {
		t.Fatalf("wine invoked without the systray registry key\nwant: %s\ngot:  %s", want, got)
	}
}

func TestResolveBinariesUsesOverrideDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"wine", "wineboot", "wineserver"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))

	wine, wineboot, wineserver, err := resolveBinaries()
	if err != nil {
		t.Fatal(err)
	}
	if wine != filepath.Join(dir, "wine") || wineboot != filepath.Join(dir, "wineboot") || wineserver != filepath.Join(dir, "wineserver") {
		t.Fatalf("unexpected binaries: %q %q %q", wine, wineboot, wineserver)
	}
}

// fakeWineBinaries writes stub wine/wineboot/wineserver scripts that append their
// arguments to an invocation log, and points the adapter at them. The wineserver
// stub exits with wineserverExit, which is 1 when no wineserver is running for the
// prefix. It returns the path of that log.
func fakeWineBinaries(t *testing.T, wineserverExit int) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocation.log")

	wine := filepath.Join(dir, "wine")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ]; then echo 'wine-10.0'; exit 0; fi\n" +
		"printf 'wine %s\\n' \"$*\" >> '" + logPath + "'\n"
	if err := os.WriteFile(wine, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	wineserver := filepath.Join(dir, "wineserver")
	serverScript := "#!/bin/sh\n" +
		"printf 'wineserver %s\\n' \"$*\" >> '" + logPath + "'\n" +
		fmt.Sprintf("exit %d\n", wineserverExit)
	if err := os.WriteFile(wineserver, []byte(serverScript), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "wineboot"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(wineOverrideEnv, wine)
	return logPath
}

// TestGracefulKillAsksBattleNetToCloseFirst guards the session-persistence fix:
// Battle.net must be asked to close its windows before the wineserver is stopped.
// Killing it outright leaves no chance to save the session, and the next start
// then needs an interactive login, which does not complete under Wine Wayland.
func TestGracefulKillAsksBattleNetToCloseFirst(t *testing.T) {
	logPath := fakeWineBinaries(t, 0)

	oldWait := battleNetCloseWait
	battleNetCloseWait = 0
	t.Cleanup(func() { battleNetCloseWait = oldWait })

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.GracefulKillPrefix(t.TempDir(), time.Second); err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read invocation log: %v", err)
	}

	calls := string(got)
	closeIdx := strings.Index(calls, "wine taskkill /IM Battle.net.exe")
	killIdx := strings.Index(calls, "wineserver -k\n")
	if closeIdx < 0 {
		t.Fatalf("Battle.net was not asked to close:\n%s", calls)
	}
	if killIdx < 0 {
		t.Fatalf("wineserver was never stopped:\n%s", calls)
	}
	if closeIdx > killIdx {
		t.Fatalf("wineserver was stopped before Battle.net was asked to close:\n%s", calls)
	}
}

// TestGracefulKillReapsLeftoversWithoutWineserver guards the leftover fix: a prefix
// whose wineserver is gone still holds wine processes, and a shutdown used to return
// as soon as `wineserver -k` failed, leaving them running forever.
func TestGracefulKillReapsLeftoversWithoutWineserver(t *testing.T) {
	// wineserver exits 1 on every call, which is what wine reports when the prefix
	// has no server left.
	fakeWineBinaries(t, 1)

	prefixPath := t.TempDir()
	leftover := startFakeWineProcess(t, "sleep", "WINEPREFIX="+winePrefixPath(prefixPath), "WINESERVERSOCKET=/tmp/server.sock")

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.GracefulKillPrefix(prefixPath, time.Second); err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	waitForExit(t, leftover)
}

// TestGracefulKillReapsWindowsNamedLeftovers covers the other half of the process
// scan: wine names Windows processes after the executable it runs, and the name
// alone must be enough to recognize them.
func TestGracefulKillReapsWindowsNamedLeftovers(t *testing.T) {
	fakeWineBinaries(t, 1)

	prefixPath := t.TempDir()
	leftover := startFakeWineProcess(t, "leftover.exe", "WINEPREFIX="+winePrefixPath(prefixPath))

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.GracefulKillPrefix(prefixPath, time.Second); err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	waitForExit(t, leftover)
}

// TestGracefulKillLeavesForeignProcessesAlone guards against killing whatever else
// inherits WINEPREFIX, for example a terminal or editor started from a shell with
// WINEPREFIX exported.
func TestGracefulKillLeavesForeignProcessesAlone(t *testing.T) {
	fakeWineBinaries(t, 1)

	prefixPath := t.TempDir()
	foreign := startFakeWineProcess(t, "sleep", "WINEPREFIX="+winePrefixPath(prefixPath))

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.GracefulKillPrefix(prefixPath, time.Second); err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	requireAlive(t, foreign)
}

// TestGracefulKillLeavesNeighbouringPrefixesAlone guards the exact environment
// match: "WINEPREFIX=<prefix>/pfx-backup" must not be mistaken for the prefix.
func TestGracefulKillLeavesNeighbouringPrefixesAlone(t *testing.T) {
	fakeWineBinaries(t, 1)

	prefixPath := t.TempDir()
	neighbour := startFakeWineProcess(t, "leftover.exe", "WINEPREFIX="+winePrefixPath(prefixPath)+"-backup")

	adapter := NewAdapter(log.New(io.Discard))
	if err := adapter.GracefulKillPrefix(prefixPath, time.Second); err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	requireAlive(t, neighbour)
}

// winePrefixPath returns the prefix directory a wine process reports as WINEPREFIX.
func winePrefixPath(prefixPath string) string {
	return filepath.Join(prefixPath, "pfx")
}

// startFakeWineProcess starts a long-running process that the prefix scan accepts as
// part of a wine session, either through wine's environment (env) or through a
// Windows executable name. A name other than "sleep" runs a copy of the sleep
// binary, so /proc/<pid>/comm matches it.
func startFakeWineProcess(t *testing.T, name string, env ...string) *exec.Cmd {
	t.Helper()

	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("sleep is unavailable: %v", err)
	}

	exe := sleep
	if name != filepath.Base(sleep) {
		data, err := os.ReadFile(sleep)
		if err != nil {
			t.Fatal(err)
		}
		exe = filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(exe, data, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(exe, "60")
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", name, err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd
}

// waitForExit fails the test unless the process exits on its own.
func waitForExit(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("process %d survived the kill", cmd.Process.Pid)
	}
}

// requireAlive fails the test when a process that had to be left alone is gone.
func requireAlive(t *testing.T, cmd *exec.Cmd) {
	t.Helper()

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", cmd.Process.Pid))
	if err != nil {
		t.Fatalf("process %d was killed: %v", cmd.Process.Pid, err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 || fields[2] == "Z" {
		t.Fatalf("process %d is not running: %s", cmd.Process.Pid, string(data))
	}
}
