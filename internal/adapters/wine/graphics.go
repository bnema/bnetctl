package wine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bnema/bnetctl/internal/domain"
)

type graphicsDLL struct{ source, destination, name string }

// Discover package data beside the resolved setup script. Arch's vkd3d command
// is a wrapper, so its package share directory is the fallback. Never run it.
func graphicsBase(script, fallback string) string {
	resolved, err := filepath.EvalSymlinks(script)
	if err == nil {
		base := filepath.Dir(resolved)
		if info, err := os.Stat(filepath.Join(base, "x64")); err == nil && info.IsDir() {
			return base
		}
	}
	return fallback
}
func graphicsPlan(rt *domain.WineRuntime, pfx string) ([]graphicsDLL, error) {
	var result []graphicsDLL
	for _, pkg := range []struct {
		base, lib32 string
		names       []string
	}{
		{graphicsBase(rt.DXVKSetupBin, "/usr/share/dxvk"), "x32", []string{"dxgi", "d3d8", "d3d9", "d3d10core", "d3d11"}},
		{graphicsBase(rt.VKD3DSetupBin, "/usr/share/vkd3d-proton"), "x86", []string{"d3d12", "d3d12core"}},
	} {
		for _, arch := range []struct{ lib, dir string }{{"x64", "system32"}, {pkg.lib32, "syswow64"}} {
			for _, name := range pkg.names {
				src := filepath.Join(pkg.base, arch.lib, name+".dll")
				info, err := os.Stat(src)
				if err != nil {
					return nil, fmt.Errorf("graphics DLL %s: %w", src, err)
				}
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("not a graphics DLL file: %s", src)
				}
				result = append(result, graphicsDLL{src, filepath.Join(pfx, "drive_c", "windows", arch.dir, name+".dll"), name})
			}
		}
	}
	return result, nil
}

const overridesKey = `HKCU\Software\Wine\DllOverrides`

func (a *Adapter) installGraphics(rt *domain.WineRuntime, env map[string]string) error {
	plan, err := graphicsPlan(rt, env["WINEPREFIX"])
	if err != nil {
		return err
	}
	for _, dll := range plan {
		if err := os.MkdirAll(filepath.Dir(dll.destination), 0755); err != nil {
			return err
		}
		if _, err := os.Lstat(dll.destination); err == nil {
			// Preserve originals just as the package scripts do.
			if _, err := os.Lstat(dll.destination + ".old"); os.IsNotExist(err) {
				if err := os.Rename(dll.destination, dll.destination+".old"); err != nil {
					return err
				}
			} else if err := os.Remove(dll.destination); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Symlink(dll.source, dll.destination); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, dll := range plan {
		if seen[dll.name] {
			continue
		}
		seen[dll.name] = true
		if err := runPrefixCommand(rt.WineBin, env, "reg", "add", overridesKey, "/v", dll.name, "/t", "REG_SZ", "/d", "native", "/f"); err != nil {
			return err
		}
	}
	if err := verifyGraphics(rt, env, plan); err != nil {
		return err
	}
	return runPrefixCommand(rt.WineServerBin, env, "-w")
}
func verifyGraphics(rt *domain.WineRuntime, env map[string]string, plan []graphicsDLL) error {
	seen := map[string]bool{}
	for _, dll := range plan {
		target, err := os.Readlink(dll.destination)
		if err != nil {
			return fmt.Errorf("verify graphics symlink %s: %w", dll.destination, err)
		}
		if target != dll.source {
			return fmt.Errorf("unexpected graphics symlink target: %s -> %s", dll.destination, target)
		}
		if _, err := os.Stat(dll.destination); err != nil {
			return fmt.Errorf("broken graphics symlink %s: %w", dll.destination, err)
		}
		if seen[dll.name] {
			continue
		}
		seen[dll.name] = true
		cmd := exec.Command(rt.WineBin, "reg", "query", overridesKey, "/v", dll.name)
		cmd.Env = envMapToSlice(env)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("verify native override %s: %w (%s)", dll.name, err, out)
		}
		found := false
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 3 && strings.EqualFold(fields[0], dll.name) && fields[1] == "REG_SZ" && fields[2] == "native" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("native override not verified for %s: %s", dll.name, out)
		}
	}
	// Registry commands must not have removed DLLs verified earlier in the loop.
	for _, dll := range plan {
		target, err := os.Readlink(dll.destination)
		if err != nil {
			return fmt.Errorf("verify graphics symlink %s: %w", dll.destination, err)
		}
		if target != dll.source {
			return fmt.Errorf("unexpected graphics symlink target: %s", dll.destination)
		}
		if _, err := os.Stat(dll.destination); err != nil {
			return fmt.Errorf("broken graphics symlink %s: %w", dll.destination, err)
		}
	}
	return nil
}
