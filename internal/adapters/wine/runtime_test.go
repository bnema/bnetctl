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
// prefix, and runs wineserverExtra before exiting, so a test can emulate a
// wineserver taking its clients down. It returns the path of that log.
func fakeWineBinaries(t *testing.T, wineserverExit int, wineserverExtra string) string {
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
		wineserverExtra +
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
	logPath := fakeWineBinaries(t, 0, "")

	oldWait := battleNetCloseWait
	battleNetCloseWait = 0
	t.Cleanup(func() { battleNetCloseWait = oldWait })

	adapter := NewAdapter(log.New(io.Discard))
	if _, err := adapter.GracefulKillPrefix(t.TempDir(), time.Second); err != nil {
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
	fakeWineBinaries(t, 1, "")

	prefixPath := t.TempDir()
	leftover := startFakeWineProcess(t, "sleep", "WINEPREFIX="+winePrefixPath(prefixPath), "WINESERVERSOCKET=/tmp/server.sock")

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(prefixPath, time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}
	requireStopped(t, result, leftover, "sleep")

	waitForExit(t, leftover)
}

// TestGracefulKillReapsWindowsNamedLeftovers covers the other half of the process
// scan: wine names Windows processes after the executable it runs, and the name
// alone must be enough to recognize them.
func TestGracefulKillReapsWindowsNamedLeftovers(t *testing.T) {
	fakeWineBinaries(t, 1, "")

	prefixPath := t.TempDir()
	leftover := startFakeWineProcess(t, "leftover.exe", "WINEPREFIX="+winePrefixPath(prefixPath))

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(prefixPath, time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}
	requireStopped(t, result, leftover, "leftover.exe")

	waitForExit(t, leftover)
}

// TestGracefulKillLeavesForeignProcessesAlone guards against killing whatever else
// inherits WINEPREFIX, for example a terminal or editor started from a shell with
// WINEPREFIX exported.
func TestGracefulKillLeavesForeignProcessesAlone(t *testing.T) {
	fakeWineBinaries(t, 1, "")

	prefixPath := t.TempDir()
	foreign := startFakeWineProcess(t, "sleep", "WINEPREFIX="+winePrefixPath(prefixPath))

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(prefixPath, time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}
	requireNothingStopped(t, result)

	requireAlive(t, foreign)
}

// TestGracefulKillLeavesNeighbouringPrefixesAlone guards the exact environment
// match: "WINEPREFIX=<prefix>/pfx-backup" must not be mistaken for the prefix.
func TestGracefulKillLeavesNeighbouringPrefixesAlone(t *testing.T) {
	fakeWineBinaries(t, 1, "")

	prefixPath := t.TempDir()
	neighbour := startFakeWineProcess(t, "leftover.exe", "WINEPREFIX="+winePrefixPath(prefixPath)+"-backup")

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(prefixPath, time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}
	requireNothingStopped(t, result)

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

	if len(name) > 15 {
		t.Fatalf("process name %q exceeds the 15 bytes the kernel keeps in comm", name)
	}

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

// requireStopped fails the test unless the result reports exactly the given process.
func requireStopped(t *testing.T, result domain.StopResult, cmd *exec.Cmd, name string) {
	t.Helper()

	if result.Wineserver != nil || len(result.Session) != 0 {
		t.Fatalf("no wineserver was running, got %+v", result)
	}
	if len(result.Leftovers) != 1 {
		t.Fatalf("expected 1 stopped process, got %+v", result.Leftovers)
	}
	if stopped := result.Leftovers[0]; stopped.PID != cmd.Process.Pid || stopped.Name != name {
		t.Fatalf("expected %s (pid %d), got %s", name, cmd.Process.Pid, stopped)
	}
}

// requireNothingStopped fails the test when a stop reports work it should not have
// done.
func requireNothingStopped(t *testing.T, result domain.StopResult) {
	t.Helper()

	if result.Wineserver != nil || len(result.Session) != 0 || len(result.Leftovers) != 0 {
		t.Fatalf("expected nothing to be stopped, got %+v", result)
	}
}

// TestExcluding guards the report: a process that accepted SIGKILL but is still
// running must never be listed as stopped.
func TestExcluding(t *testing.T) {
	reported := domain.StoppedProcesses{{PID: 1, Name: "explorer.exe"}, {PID: 2, Name: "services.exe"}}

	kept := excluding(reported, pidSet(domain.StoppedProcesses{{PID: 2, Name: "services.exe"}}))
	if len(kept) != 1 || kept[0].PID != 1 {
		t.Fatalf("expected only pid 1 to be reported, got %+v", kept)
	}
	if got := excluding(reported, nil); len(got) != 2 {
		t.Fatalf("expected both processes to be reported, got %+v", got)
	}
	if got := excluding(nil, pidSet(reported)); len(got) != 0 {
		t.Fatalf("expected nothing to be reported, got %+v", got)
	}
}

// TestSplitWineserver guards the session report: the wineserver is reported on its
// own and the clients are listed without it.
func TestSplitWineserver(t *testing.T) {
	clients, server := splitWineserver(domain.StoppedProcesses{
		{PID: 1, Name: "wineserver"},
		{PID: 2, Name: "explorer.exe"},
	})

	if server == nil || server.PID != 1 {
		t.Fatalf("expected the wineserver (pid 1), got %v", server)
	}
	if len(clients) != 1 || clients[0].PID != 2 {
		t.Fatalf("expected only the client to be listed, got %+v", clients)
	}

	clients, server = splitWineserver(nil)
	if server != nil || len(clients) != 0 {
		t.Fatalf("expected nothing to be split, got %v and %+v", server, clients)
	}
}

// TestGracefulKillReportsTheStoppedSession guards the session report end to end: the
// snapshot taken before the stop names the clients the wineserver takes down, and
// reports the wineserver itself.
func TestGracefulKillReportsTheStoppedSession(t *testing.T) {
	prefixPath := t.TempDir()
	client := startFakeWineProcess(t, "client.exe", "WINEPREFIX="+winePrefixPath(prefixPath), "WINESERVERSOCKET=/tmp/server.sock")
	server := startFakeWineProcess(t, "wineserver", "WINEPREFIX="+winePrefixPath(prefixPath))

	// The stub acts as the wineserver would on -k: it takes its clients down with it.
	stopClients := fmt.Sprintf("if [ \"$1\" = \"-k\" ]; then kill -9 %d %d 2>/dev/null; fi\n", server.Process.Pid, client.Process.Pid)
	fakeWineBinaries(t, 0, stopClients)

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(prefixPath, time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	if result.Wineserver == nil || result.Wineserver.PID != server.Process.Pid {
		t.Fatalf("expected the wineserver (pid %d), got %+v", server.Process.Pid, result.Wineserver)
	}
	if len(result.Session) != 1 || result.Session[0].PID != client.Process.Pid || result.Session[0].Name != "client.exe" {
		t.Fatalf("expected the client to be reported as stopped, got %+v", result.Session)
	}
	if len(result.Leftovers) != 0 {
		t.Fatalf("nothing survived to the sweep, got %+v", result.Leftovers)
	}
}

// TestGracefulKillReportsWineserverWithoutSnapshot guards the report when a
// wineserver answers but no process of the prefix was observed: the stop still knows
// a server was running and must not claim nothing was.
func TestGracefulKillReportsWineserverWithoutSnapshot(t *testing.T) {
	// -k exits 0, which is what wine reports when a server answered.
	fakeWineBinaries(t, 0, "")

	adapter := NewAdapter(log.New(io.Discard))
	result, err := adapter.GracefulKillPrefix(t.TempDir(), time.Second)
	if err != nil {
		t.Fatalf("GracefulKillPrefix: %v", err)
	}

	if result.Wineserver == nil || result.Wineserver.Name != wineServerProcessName {
		t.Fatalf("expected a wineserver entry, got %+v", result.Wineserver)
	}
	if len(result.Session) != 0 || len(result.Leftovers) != 0 {
		t.Fatalf("expected nothing else to be reported, got %+v", result)
	}
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

	status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", cmd.Process.Pid))
	if err != nil {
		t.Fatalf("process %d was killed: %v", cmd.Process.Pid, err)
	}
	// The state has a line of its own, so a process name containing spaces (wine
	// truncates "Battle.net Helper.exe" to "Battle.net Help") cannot shift it.
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "State:") && strings.ContainsAny(line, "Z") {
			t.Fatalf("process %d is a zombie", cmd.Process.Pid)
		}
	}
}
