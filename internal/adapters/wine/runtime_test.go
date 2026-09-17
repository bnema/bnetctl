package wine

import (
	"io"
	"os"
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
// arguments to an invocation log, and points the adapter at them. It returns the
// path of that log.
func fakeWineBinaries(t *testing.T) string {
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
		"printf 'wineserver %s\\n' \"$*\" >> '" + logPath + "'\n"
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
	logPath := fakeWineBinaries(t)

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
