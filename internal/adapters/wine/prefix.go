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
	return writeVersionRecord(prefix, prefixVersionFile, version)
}
func writeVersionRecord(prefix, name, version string) error {
	path := filepath.Join(prefix, name)
	f, err := os.CreateTemp(prefix, ".wine-version-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err := f.WriteString(version + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	dir, err := os.Open(prefix)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
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
	pending, err := readVersion(filepath.Join(prefix, prefixVersionFile+".migrating"))
	if err != nil {
		return err
	}
	for _, minimum := range []string{from, pending} {
		if minimum != "" {
			cmp, err := compareVersions(rt.Version, minimum)
			if err != nil {
				return err
			}
			if cmp < 0 {
				return fmt.Errorf("refusing Wine downgrade: selected %s; prefix requires >= %s", rt.Version, minimum)
			}
		}
	}
	if from == rt.Version && pending == "" {
		return nil
	}
	if err := checkRunOnce(filepath.Join(prefix, "pfx")); err != nil {
		return err
	}
	if _, err := graphicsPlan(rt, filepath.Join(prefix, "pfx")); err != nil {
		return err
	}
	if a.IsProcessRunning(prefix) {
		return fmt.Errorf("cannot update Wine prefix while it is running")
	}
	if err := writeVersionRecord(prefix, prefixVersionFile+".migrating", rt.Version); err != nil {
		return err
	}
	a.log.Info("wine version changed, updating prefix", "from", from, "to", rt.Version)
	env := a.buildEnv(filepath.Join(prefix, "pfx"), wineEnv)
	for _, step := range []struct {
		bin  string
		args []string
	}{{rt.WineBootBin, []string{"-u", "-r"}}, {rt.WineServerBin, []string{"-w"}}} {
		if err := runPrefixCommand(step.bin, env, step.args...); err != nil {
			return fmt.Errorf("update Wine prefix: %w", err)
		}
	}
	if err := a.installGraphics(rt, env); err != nil {
		return fmt.Errorf("restore prefix graphics: %w", err)
	}
	if err := writePrefixVersion(prefix, rt.Version); err != nil {
		return err
	}
	return os.Remove(filepath.Join(prefix, prefixVersionFile+".migrating"))
}

func runPrefixCommand(bin string, env map[string]string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = envMapToSlice(env)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w (%s)", bin, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
