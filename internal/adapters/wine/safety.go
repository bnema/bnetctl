package wine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func versionNumbers(v string) ([]int, error) {
	if !strings.HasPrefix(v, "wine-") {
		return nil, fmt.Errorf("invalid Wine version %q", v)
	}
	v = strings.TrimPrefix(v, "wine-")
	end := 0
	for end < len(v) && (v[end] >= '0' && v[end] <= '9' || v[end] == '.') {
		end++
	}
	if end < len(v) && v[end] != '-' && v[end] != ' ' {
		return nil, fmt.Errorf("invalid Wine version %q", v)
	}
	fields := strings.Split(v[:end], ".")
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid Wine version %q", v)
	}
	result := make([]int, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, err
		}
		result[i] = n
	}
	return result, nil
}
func compareVersions(a, b string) (int, error) {
	x, err := versionNumbers(a)
	if err != nil {
		return 0, err
	}
	y, err := versionNumbers(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < len(x) || i < len(y); i++ {
		u, v := 0, 0
		if i < len(x) {
			u = x[i]
		}
		if i < len(y) {
			v = y[i]
		}
		if u < v {
			return -1, nil
		}
		if u > v {
			return 1, nil
		}
	}
	return 0, nil
}
func companionBinaries(wine string) (string, string, error) {
	boot, server := filepath.Join(filepath.Dir(wine), "wineboot"), filepath.Join(filepath.Dir(wine), "wineserver")
	if !isExecutable(boot) || !isExecutable(server) {
		return "", "", fmt.Errorf("missing companion binaries beside %s", wine)
	}
	return boot, server, nil
}
func chooseMinimumWine(selected, version, minimum string) (string, string, error) {
	cmp, err := compareVersions(version, minimum)
	if err != nil {
		return "", "", err
	}
	if cmp >= 0 {
		return selected, version, nil
	}
	if os.Getenv(wineOverrideEnv) != "" {
		return "", "", fmt.Errorf("refusing Wine downgrade: BNETCTL_WINE is %s; prefix requires >= %s", version, minimum)
	}
	path, _ := exec.LookPath("wine")
	for _, candidate := range []string{path, cachyOSWineBinary} {
		if candidate == "" || !isExecutable(candidate) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		out, err := exec.Command(resolved, "--version").Output()
		if err != nil {
			continue
		}
		v := strings.TrimSpace(string(out))
		cmp, err := compareVersions(v, minimum)
		if err == nil && cmp >= 0 {
			if _, _, err := companionBinaries(resolved); err == nil {
				return resolved, v, nil
			}
		}
	}
	return "", "", fmt.Errorf("refusing Wine downgrade: selected %s; prefix requires >= %s; set BNETCTL_WINE", version, minimum)
}
func readVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(data))
	if _, err := versionNumbers(v); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}
func (a *Adapter) LockPrefix(prefix string) (func(), error) {
	release, err := a.lockPrefixExclusion(prefix, 0)
	if err != nil {
		return nil, err
	}
	versions, err := a.selectPrefixRuntime(prefix)
	if err != nil {
		release()
		return nil, err
	}
	rt := a.detected
	if versions[prefixVersionFile+".migrating"] != "" || versions[prefixVersionFile] != rt.Version {
		if err := checkRunOnce(filepath.Join(prefix, "pfx")); err != nil {
			release()
			return nil, err
		}
		// Only an uncommitted initial creation can safely retry --init without
		// an architecture header. Established prefixes must still validate it.
		if versions[prefixVersionFile] == "" && versions[prefixVersionFile+".migrating"] != "" {
			if _, err := os.Stat(filepath.Join(prefix, "pfx", "system.reg")); os.IsNotExist(err) {
				if a.IsProcessRunning(prefix) {
					release()
					return nil, fmt.Errorf("cannot retry Wine prefix initialization while it is running")
				}
				if err := a.CreatePrefix(prefix); err != nil {
					release()
					return nil, fmt.Errorf("retry Wine prefix initialization: %w", err)
				}
			} else if err != nil {
				release()
				return nil, err
			}
		}
		// New prefixes have no system.reg until wineboot initializes them.
		if _, err := os.Stat(filepath.Join(prefix, "pfx")); err == nil {
			if _, err := readPrefixArch(filepath.Join(prefix, "pfx")); err != nil {
				release()
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			release()
			return nil, err
		}
	}
	return release, nil
}

// selectPrefixRuntime applies the recorded version floor without migration preflight.
func (a *Adapter) selectPrefixRuntime(prefix string) (map[string]string, error) {
	a.minimum = ""
	versions := map[string]string{}
	for _, name := range []string{prefixVersionFile, prefixVersionFile + ".migrating"} {
		v, err := readVersion(filepath.Join(prefix, name))
		if err != nil {
			return nil, err
		}
		versions[name] = v
		if v != "" {
			cmp := 1
			if a.minimum != "" {
				cmp, err = compareVersions(v, a.minimum)
			}
			if err != nil {
				return nil, err
			}
			if cmp > 0 {
				a.minimum = v
			}
		}
	}
	a.detected = nil
	_, err := a.Detect()
	if err != nil {
		return nil, err
	}
	return versions, nil
}

// lockPrefixExclusion never runs Wine or migration preflight. The persistent
// sibling inode must not be removed: waiters must all lock the same file even
// while the prefix is deleted and recreated.
func (a *Adapter) lockPrefixExclusion(prefix string, wait time.Duration) (func(), error) {
	prefix = filepath.Clean(prefix)
	if err := os.MkdirAll(filepath.Dir(prefix), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(prefix+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	announced := false
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || !time.Now().Before(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("prefix is locked by another bnetctl invocation (retry after maintenance completes): %w", err)
		}
		if !announced {
			a.log.Info("waiting for prefix maintenance lock", "prefix", prefix, "timeout", wait)
			announced = true
		}
		time.Sleep(25 * time.Millisecond)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }) }, nil
}
