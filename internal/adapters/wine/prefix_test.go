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

func migrationFixture(t *testing.T) (*Adapter, string, string) {
	t.Helper()
	dir := t.TempDir()
	prefix := filepath.Join(dir, "prefix")
	mustWrite(t, filepath.Join(prefix, "pfx", "system.reg"), "WINE REGISTRY Version 2\n")
	calls := filepath.Join(dir, "calls")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = '-k0' ]; then exit 1; fi
echo "$(basename "$0") $* HDR=$DXVK_HDR DISPLAY=${DISPLAY-unset}" >> '%s'
if [ "$1" = 'reg' ] && [ "$2" = 'query' ]; then echo "$5 REG_SZ native"; fi
`, calls)
	for _, name := range []string{"wine", "wineboot", "wineserver"} {
		mustWrite(t, filepath.Join(dir, name), script)
	}
	rt := &domain.WineRuntime{WineBin: filepath.Join(dir, "wine"), WineBootBin: filepath.Join(dir, "wineboot"), WineServerBin: filepath.Join(dir, "wineserver"), Version: "wine-11.21"}
	for _, pkg := range []struct {
		name, lib32 string
		dlls        []string
	}{{"dxvk", "x32", []string{"dxgi", "d3d8", "d3d9", "d3d10core", "d3d11"}}, {"vkd3d", "x86", []string{"d3d12", "d3d12core"}}} {
		base := filepath.Join(dir, pkg.name)
		setup := filepath.Join(base, "setup")
		mustWrite(t, setup, "#!/bin/sh\nexit 99\n")
		if pkg.name == "dxvk" {
			rt.DXVKSetupBin = setup
		} else {
			rt.VKD3DSetupBin = setup
		}
		for _, arch := range []string{"x64", pkg.lib32} {
			for _, dll := range pkg.dlls {
				mustWrite(t, filepath.Join(base, arch, dll+".dll"), "dll")
			}
		}
	}
	a := NewAdapter(log.New(io.Discard))
	a.detected = rt
	return a, prefix, calls
}
func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestPrefixMigration(t *testing.T) {
	for _, tt := range []struct{ name, old, pending, fail string }{
		{"missing", "", "", ""}, {"same", "wine-11.21", "", ""}, {"changed", "wine-10.0", "", ""},
		{"recovery", "wine-11.21", "wine-11.21", ""}, {"boot failure", "wine-10.0", "", "wineboot"},
		{"wait failure", "wine-10.0", "", "wineserver"}, {"override failure", "wine-10.0", "", "wine"},
		{"missing DLL despite zero exit", "wine-10.0", "", "delete"}, {"unverified override", "wine-10.0", "", "query"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, prefix, calls := migrationFixture(t)
			marker := filepath.Join(prefix, prefixVersionFile)
			if tt.old != "" {
				mustWrite(t, marker, tt.old+"\n")
			}
			if tt.pending != "" {
				mustWrite(t, marker+".migrating", tt.pending+"\n")
			}
			if tt.fail != "" {
				if tt.fail == "delete" {
					mustWrite(t, a.detected.WineBin, "#!/bin/sh\nrm -f \"$WINEPREFIX/drive_c/windows/system32/dxgi.dll\"\nexit 0\n")
				} else if tt.fail == "query" {
					mustWrite(t, a.detected.WineBin, "#!/bin/sh\nexit 0\n")
				} else {
					mustWrite(t, filepath.Join(filepath.Dir(calls), tt.fail), "#!/bin/sh\nif [ \"$1\" = '-k0' ]; then exit 1; fi\nexit 7\n")
				}
			}
			t.Setenv("DISPLAY", ":1")
			err := a.UpdatePrefix(prefix, &domain.WineEnv{Vars: map[string]string{"DXVK_HDR": "1"}, Unset: []string{"DISPLAY"}})
			if (err != nil) != (tt.fail != "") {
				t.Fatalf("result: %v", err)
			}
			data, readErr := os.ReadFile(marker)
			pending, pendingErr := os.ReadFile(marker + ".migrating")
			if tt.fail != "" {
				if strings.TrimSpace(string(data)) != tt.old {
					t.Fatalf("marker changed: %s", data)
				}
				if pendingErr != nil || strings.TrimSpace(string(pending)) != "wine-11.21" {
					t.Fatalf("missing recovery: %s %v", pending, pendingErr)
				}
			} else {
				if readErr != nil || string(data) != "wine-11.21\n" {
					t.Fatalf("marker: %s %v", data, readErr)
				}
				if !os.IsNotExist(pendingErr) {
					t.Fatal("recovery record not removed")
				}
			}
			invocation, _ := os.ReadFile(calls)
			text := string(invocation)
			if tt.old == "wine-11.21" && tt.pending == "" {
				if text != "" {
					t.Fatalf("same version ran commands: %s", text)
				}
				return
			}
			if tt.fail == "" {
				for _, want := range []string{"wineboot -u -r", "wineserver -w", "/v dxgi", "/v d3d12core", "/d native", "HDR=1 DISPLAY=unset"} {
					if !strings.Contains(text, want) {
						t.Fatalf("missing %q: %s", want, text)
					}
				}
				plan, err := graphicsPlan(a.detected, filepath.Join(prefix, "pfx"))
				if err != nil {
					t.Fatal(err)
				}
				if err := verifyGraphics(a.detected, a.buildEnv(filepath.Join(prefix, "pfx"), nil), plan); err != nil {
					t.Fatal(err)
				}
			}
		})
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
