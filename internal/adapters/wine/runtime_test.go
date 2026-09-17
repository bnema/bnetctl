package wine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
