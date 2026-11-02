package wine

import (
	"bytes"
	"github.com/charmbracelet/log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportsWaylandHDR(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{"wine-11.19", true}, {"wine-11.21", true}, {"wine-12.0", true},
		{"wine-10.0-20260425 (CachyOS)", false}, {"wine-11.19-20260425 (CachyOS)", true},
		{"wine-11.19 (Staging)", true}, {"wine-11.18 (Staging)", false},
		{"", false}, {"garbage", false}, {"wine-11", false}, {"wine-11.19oops", false},
		{"wine-9999999999999999999999999.0", false},
	} {
		t.Run(tt.version, func(t *testing.T) {
			if got := supportsWaylandHDR(tt.version); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathWinePreferredInAllModes(t *testing.T) {
	dir := fakeVersionInstall(t, "wine-11.21")
	t.Setenv("PATH", dir)
	t.Setenv(wineOverrideEnv, "")
	for _, wayland := range []bool{false, true} {
		wine, boot, server, err := resolveBinariesForWayland(wayland)
		if err != nil {
			t.Fatal(err)
		}
		if wine != filepath.Join(dir, "wine") || boot != filepath.Join(dir, "wineboot") || server != filepath.Join(dir, "wineserver") {
			t.Fatalf("selection: %s %s %s", wine, boot, server)
		}
	}
	override := fakeVersionInstall(t, "wine-11.19")
	t.Setenv(wineOverrideEnv, filepath.Join(override, "wine"))
	for _, wayland := range []bool{false, true} {
		wine, _, _, err := resolveBinariesForWayland(wayland)
		if err != nil || wine != filepath.Join(override, "wine") {
			t.Fatalf("override: %s %v", wine, err)
		}
	}
}

func TestWaylandOverrideAndCachedDetection(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"wine", "wineboot", "wineserver"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho wine-10.0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(wineOverrideEnv, filepath.Join(dir, "wine"))
	var output bytes.Buffer
	adapter := NewWaylandAdapter(log.New(&output))
	first, err := adapter.Detect()
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Detect()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("runtime was not cached")
	}
	if first.WineBin != filepath.Join(dir, "wine") || first.WineBootBin != filepath.Join(dir, "wineboot") || first.WineServerBin != filepath.Join(dir, "wineserver") {
		t.Fatalf("override not preserved: %+v", first)
	}
	if strings.Count(output.String(), "selected wine runtime") != 1 {
		t.Fatalf("expected one selection log: %s", output.String())
	}
	if !strings.Contains(output.String(), "HDR on Wayland needs Wine >= 11.19 (found wine-10.0); set BNETCTL_WINE") {
		t.Fatalf("missing warning: %s", output.String())
	}
}
