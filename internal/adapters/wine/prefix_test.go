package wine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/charmbracelet/log"
)

func TestPrefixMigration(t *testing.T) {
	for _, tt := range []struct {
		name, old, fail string
		changed         bool
	}{
		{"missing", "", "", true}, {"same", "wine-11.21", "", false}, {"changed", "wine-10.0", "", true},
		{"boot failure", "wine-10.0", "wineboot", true}, {"wait failure", "wine-10.0", "wineserver", true},
		{"dxvk failure", "wine-10.0", "setup_dxvk", true}, {"vkd3d failure", "wine-10.0", "setup_vkd3d_proton", true},
		{"override failure", "wine-10.0", "wine", true}, {"missing failure", "", "wineboot", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			prefix := filepath.Join(dir, "prefix")
			if err := os.MkdirAll(filepath.Join(prefix, "pfx"), 0755); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(prefix, prefixVersionFile)
			if tt.old != "" {
				if err := os.WriteFile(marker, []byte(tt.old+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			calls := filepath.Join(dir, "calls")
			for _, name := range []string{"wine", "wineboot", "wineserver", "setup_dxvk", "setup_vkd3d_proton"} {
				script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = '-k0' ]; then exit 1; fi\necho '%s' \"$* HDR=$DXVK_HDR DISPLAY=${DISPLAY-unset} PREFIX=$WINEPREFIX\" >> '%s'\n", name, calls)
				// Every command observes the old marker, proving it is not committed early.
				script += fmt.Sprintf("if [ -f '%s' ] && [ \"$(cat '%s')\" = 'wine-11.21' ]; then exit 99; fi\n", marker, marker)
				if name == tt.fail {
					script += "exit 7\n"
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
					t.Fatal(err)
				}
			}
			a := NewAdapter(log.New(io.Discard))
			a.detected = &domain.WineRuntime{
				WineBin: filepath.Join(dir, "wine"), WineBootBin: filepath.Join(dir, "wineboot"), WineServerBin: filepath.Join(dir, "wineserver"), Version: "wine-11.21",
				HasDXVKSetup: true, DXVKSetupBin: filepath.Join(dir, "setup_dxvk"), HasVKD3DSetup: true, VKD3DSetupBin: filepath.Join(dir, "setup_vkd3d_proton")}
			t.Setenv("DISPLAY", ":1")
			err := a.UpdatePrefix(prefix, &domain.WineEnv{Vars: map[string]string{"DXVK_HDR": "1"}, Unset: []string{"DISPLAY"}})
			if (err != nil) != (tt.fail != "") {
				t.Fatalf("unexpected result: %v", err)
			}
			data, readErr := os.ReadFile(marker)
			if tt.fail != "" {
				if tt.old == "" {
					if !os.IsNotExist(readErr) {
						t.Fatalf("marker created: %s %v", data, readErr)
					}
				} else if string(data) != tt.old+"\n" {
					t.Fatalf("marker changed on failure: %s", data)
				}
			} else if string(data) != "wine-11.21\n" {
				t.Fatalf("marker not committed: %s %v", data, readErr)
			}
			invocation, _ := os.ReadFile(calls)
			text := string(invocation)
			if !tt.changed && text != "" {
				t.Fatalf("same version ran commands: %s", text)
			}
			if tt.changed && tt.fail == "" {
				for _, want := range []string{"wineboot -u", "wineserver -w", "setup_dxvk install --symlink", "setup_vkd3d_proton install --symlink", "/v dxgi", "/v d3d12", "HDR=1 DISPLAY=unset PREFIX=" + filepath.Join(prefix, "pfx")} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %q: %s", want, text)
					}
				}
				if strings.Index(text, "wineboot -u") > strings.Index(text, "setup_dxvk") {
					t.Fatal("graphics installed before update")
				}
			}
		})
	}
}

func TestMissingSetupDoesNotUpdatePrefix(t *testing.T) {
	prefix := t.TempDir()
	a := NewAdapter(log.New(io.Discard))
	a.detected = &domain.WineRuntime{Version: "wine-11.21", WineServerBin: "/nonexistent/wineserver"}
	if err := a.UpdatePrefix(prefix, nil); err == nil || !strings.Contains(err.Error(), "setup_dxvk") {
		t.Fatalf("expected safe setup preflight failure, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, prefixVersionFile)); !os.IsNotExist(err) {
		t.Fatalf("marker unexpectedly exists: %v", err)
	}
}

func TestGraphicsUsesSelectedWineEnvironment(t *testing.T) {
	a := NewAdapter(log.New(io.Discard))
	a.detected = &domain.WineRuntime{WineBin: "/chosen/bin/wine", WineServerBin: "/chosen/bin/wineserver"}
	env := a.buildEnv(t.TempDir(), nil)
	if env["WINE"] != "/chosen/bin/wine" || env["WINESERVER"] != "/chosen/bin/wineserver" || !strings.HasPrefix(env["PATH"], "/chosen/bin:") {
		t.Fatalf("wrong setup environment: %v", env)
	}
}
