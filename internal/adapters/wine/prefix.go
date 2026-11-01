package wine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bnema/bnetctl/internal/domain"
)

const prefixVersionFile = "wine-version"

func writePrefixVersion(prefix, version string) error {
	path := filepath.Join(prefix, prefixVersionFile)
	if err := os.WriteFile(path+".tmp", []byte(version+"\n"), 0o644); err != nil {
		return fmt.Errorf("write Wine version marker: %w", err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return fmt.Errorf("replace Wine version marker: %w", err)
	}
	return nil
}

// UpdatePrefix updates an idle existing prefix before any application is started.
func (a *Adapter) UpdatePrefix(prefix string, wineEnv *domain.WineEnv) error {
	rt, err := a.Detect()
	if err != nil {
		return err
	}
	old, err := os.ReadFile(filepath.Join(prefix, prefixVersionFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read Wine version marker: %w", err)
	}
	from := strings.TrimSpace(string(old))
	if from == rt.Version {
		return nil
	}
	if a.IsProcessRunning(prefix) {
		return fmt.Errorf("cannot update Wine prefix while it is running")
	}
	if !rt.HasDXVKSetup || !rt.HasVKD3DSetup {
		return fmt.Errorf("cannot update Wine prefix safely: setup_dxvk and setup_vkd3d_proton must be on PATH to restore graphics DLLs")
	}
	a.log.Info("wine version changed, updating prefix", "from", from, "to", rt.Version)
	env := a.buildEnv(filepath.Join(prefix, "pfx"), wineEnv)
	for _, step := range []struct {
		bin  string
		args []string
	}{{rt.WineBootBin, []string{"-u"}}, {rt.WineServerBin, []string{"-w"}}} {
		if err := runPrefixCommand(step.bin, env, step.args...); err != nil {
			return fmt.Errorf("update Wine prefix: %w", err)
		}
	}
	if err := a.installGraphics(rt, env); err != nil {
		return fmt.Errorf("restore prefix graphics: %w", err)
	}
	return writePrefixVersion(prefix, rt.Version)
}

func runPrefixCommand(bin string, env map[string]string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = envMapToSlice(env)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w (%s)", bin, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// The setup scripts install symlinked DLLs and their registry overrides.
// Use the same strict restoration sequence for both new and upgraded prefixes.
func (a *Adapter) installGraphics(rt *domain.WineRuntime, env map[string]string) error {
	for _, setup := range []struct {
		available bool
		bin       string
	}{{rt.HasDXVKSetup, rt.DXVKSetupBin}, {rt.HasVKD3DSetup, rt.VKD3DSetupBin}} {
		if setup.available {
			if err := runPrefixCommand(setup.bin, env, "install", "--symlink"); err != nil {
				return err
			}
		}
	}
	for _, override := range []struct {
		available bool
		dll       string
	}{{rt.HasDXVKSetup, "dxgi"}, {rt.HasVKD3DSetup, "d3d12"}} {
		if override.available {
			if err := runPrefixCommand(rt.WineBin, env, "reg", "add", `HKCU\Software\Wine\DllOverrides`, "/v", override.dll, "/t", "REG_SZ", "/d", "native,builtin", "/f"); err != nil {
				return err
			}
		}
	}
	return runPrefixCommand(rt.WineServerBin, env, "-w")
}
