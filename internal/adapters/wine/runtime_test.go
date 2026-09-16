package wine

import (
	"os"
	"path/filepath"
	"testing"

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
