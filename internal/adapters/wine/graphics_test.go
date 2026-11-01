package wine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphicsPrefixArchitecture(t *testing.T) {
	for _, arch := range []string{"win32", "win64", "unknown", ""} {
		t.Run(arch, func(t *testing.T) {
			a, prefix, calls := migrationFixture(t)
			pfx := filepath.Join(prefix, "pfx")
			mustWrite(t, filepath.Join(pfx, "system.reg"), "WINE REGISTRY Version 2\n#arch="+arch+"\n")
			plan, err := graphicsPlan(a.detected, pfx)
			if arch == "unknown" || arch == "" {
				if err == nil {
					t.Fatal("accepted unknown arch")
				}
				if err := a.UpdatePrefix(prefix, nil); err == nil {
					t.Fatal("migration accepted unknown arch")
				}
				for _, path := range []string{calls, filepath.Join(prefix, prefixVersionFile+".migrating")} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("unknown arch mutated prefix: %s", path)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := 14
			if arch == "win32" {
				want = 7
			}
			if len(plan) != want {
				t.Fatalf("got %d DLLs, want %d", len(plan), want)
			}
			for _, dll := range plan {
				if arch == "win32" {
					if filepath.Base(filepath.Dir(dll.destination)) != "system32" || strings.Contains(dll.source, "/x64/") {
						t.Fatalf("wrong win32 mapping: %+v", dll)
					}
				} else if filepath.Base(filepath.Dir(dll.destination)) == "system32" && !strings.Contains(dll.source, "/x64/") {
					t.Fatalf("wrong win64 mapping: %+v", dll)
				}
			}
			if err := a.UpdatePrefix(prefix, nil); err != nil {
				t.Fatal(err)
			}
			if arch == "win32" {
				if _, err := os.Stat(filepath.Join(pfx, "drive_c/windows/syswow64")); !os.IsNotExist(err) {
					t.Fatal("win32 created syswow64")
				}
			}
		})
	}
}
func TestOptionalD3D8(t *testing.T) {
	a, prefix, calls := migrationFixture(t)
	base := filepath.Dir(a.detected.DXVKSetupBin)
	for _, arch := range []string{"x32", "x64"} {
		if err := os.Remove(filepath.Join(base, arch, "d3d8.dll")); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.UpdatePrefix(prefix, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "d3d8") {
		t.Fatalf("optional DLL registered or queried: %s", data)
	}
	for _, dir := range []string{"system32", "syswow64"} {
		if _, err := os.Lstat(filepath.Join(prefix, "pfx/drive_c/windows", dir, "d3d8.dll")); !os.IsNotExist(err) {
			t.Fatal("optional DLL installed")
		}
	}
}
func TestOptionalD3D8OtherErrorsFail(t *testing.T) {
	a, prefix, _ := migrationFixture(t)
	src := filepath.Join(filepath.Dir(a.detected.DXVKSetupBin), "x64/d3d8.dll")
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, src); err != nil {
		t.Fatal(err)
	}
	if _, err := graphicsPlan(a.detected, filepath.Join(prefix, "pfx")); err == nil {
		t.Fatal("symlink loop incorrectly treated as optional absence")
	}
}
