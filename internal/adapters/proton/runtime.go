package proton

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// Adapter implements ports.RuntimePort using proton-cachyos
type Adapter struct {
	protonPath string
}

// Ensure Adapter implements RuntimePort
var _ ports.RuntimePort = (*Adapter)(nil)

// NewAdapter creates a new Proton adapter with the given proton base path.
// If protonPath is empty, DefaultProtonPath is used.
func NewAdapter(protonPath string) *Adapter {
	if protonPath == "" {
		protonPath = domain.DefaultProtonPath
	}
	return &Adapter{protonPath: protonPath}
}

// Detect checks if proton-cachyos is installed and returns runtime info
func (a *Adapter) Detect() (*domain.ProtonRuntime, error) {
	protonScript := filepath.Join(a.protonPath, "proton")
	if _, err := os.Stat(protonScript); err != nil {
		return nil, fmt.Errorf("proton-cachyos not found at %s: %w", a.protonPath, err)
	}

	versionFile := filepath.Join(a.protonPath, "version")
	versionBytes, err := os.ReadFile(versionFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read proton version: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))

	// Determine bin directory (prefer bin-wow64 if wine doesn't exist in bin/)
	binDir := filepath.Join(a.protonPath, "files", "bin")
	if _, err := os.Stat(filepath.Join(binDir, "wine")); err != nil {
		binDir = filepath.Join(a.protonPath, "files", "bin-wow64")
	}

	return &domain.ProtonRuntime{
		BasePath:         a.protonPath,
		BinDir:           binDir + "/",
		LibDir:           filepath.Join(a.protonPath, "files", "lib") + "/",
		DistDir:          filepath.Join(a.protonPath, "files") + "/",
		DefaultPrefixDir: filepath.Join(a.protonPath, "files", "share", "default_pfx") + "/",
		Version:          version,
	}, nil
}

// CreatePrefix initializes a new Wine/Proton prefix via the proton script
func (a *Adapter) CreatePrefix(prefixPath string) error {
	env := a.buildProtonEnv(prefixPath, nil)
	env["WINEDEBUG"] = "-all"

	protonScript := filepath.Join(a.protonPath, "proton")
	cmd := exec.Command("python3", protonScript, "runinprefix", "wineboot")
	cmd.Env = envMapToSlice(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("proton prefix creation failed: %w", err)
	}
	return nil
}

// RunExe runs a Windows executable inside the prefix using proton (blocking).
func (a *Adapter) RunExe(verb string, prefixPath string, exePath string, protonEnv *domain.ProtonEnv) error {
	env := a.buildProtonEnv(prefixPath, protonEnv)
	protonScript := filepath.Join(a.protonPath, "proton")

	cmd := exec.Command("python3", protonScript, verb, exePath)
	cmd.Env = envMapToSlice(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunExeAsync runs a Windows executable inside the prefix without blocking.
// Returns a channel that receives the exit error when the process completes.
func (a *Adapter) RunExeAsync(verb string, prefixPath string, exePath string, protonEnv *domain.ProtonEnv) (<-chan error, error) {
	env := a.buildProtonEnv(prefixPath, protonEnv)
	protonScript := filepath.Join(a.protonPath, "proton")

	cmd := exec.Command("python3", protonScript, verb, exePath)
	cmd.Env = envMapToSlice(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	return done, nil
}

// IsProcessRunning checks if a Wine process is running in the given prefix.
// Uses wineserver -k0 (signal 0) which checks existence without killing.
func (a *Adapter) IsProcessRunning(prefixPath string) bool {
	runtime, err := a.Detect()
	if err != nil {
		return false
	}

	env := a.buildBaseEnv(runtime, prefixPath)
	wineserverBin := filepath.Join(runtime.BinDir, "wineserver")

	cmd := exec.Command(wineserverBin, "-k0")
	cmd.Env = envMapToSlice(env)
	return cmd.Run() == nil
}

// KillPrefix stops all Wine processes in the given prefix
func (a *Adapter) KillPrefix(prefixPath string) error {
	runtime, err := a.Detect()
	if err != nil {
		return err
	}

	env := a.buildBaseEnv(runtime, prefixPath)
	wineserverBin := filepath.Join(runtime.BinDir, "wineserver")

	cmd := exec.Command(wineserverBin, "-k")
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

// WaitPrefix blocks until the wineserver for the given prefix exits
func (a *Adapter) WaitPrefix(prefixPath string) error {
	runtime, err := a.Detect()
	if err != nil {
		return err
	}

	env := a.buildBaseEnv(runtime, prefixPath)
	wineserverBin := filepath.Join(runtime.BinDir, "wineserver")

	cmd := exec.Command(wineserverBin, "-w")
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

// GracefulKillPrefix attempts a graceful stop, waits up to timeout, then force kills.
// Also cleans up orphaned .exe processes that survive wineserver shutdown.
func (a *Adapter) GracefulKillPrefix(prefixPath string, timeout time.Duration) error {
	_ = a.KillPrefix(prefixPath)

	done := make(chan error, 1)
	go func() {
		done <- a.WaitPrefix(prefixPath)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		runtime, err := a.Detect()
		if err != nil {
			break
		}
		env := a.buildBaseEnv(runtime, prefixPath)
		wineserverBin := filepath.Join(runtime.BinDir, "wineserver")
		cmd := exec.Command(wineserverBin, "-k9")
		cmd.Env = envMapToSlice(env)
		_ = cmd.Run()
	}

	// Kill orphaned Wine processes that belong to this prefix.
	// After wineserver dies, child .exe processes (explorer.exe, services.exe, etc.)
	// can become orphans with stale systray icons.
	a.killOrphanedProcesses(prefixPath)
	return nil
}

// killOrphanedProcesses finds and kills any .exe processes whose environment
// contains STEAM_COMPAT_DATA_PATH matching our prefix.
func (a *Adapter) killOrphanedProcesses(prefixPath string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	marker := "STEAM_COMPAT_DATA_PATH=" + prefixPath

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip non-numeric dirs
		pid := entry.Name()
		if pid[0] < '1' || pid[0] > '9' {
			continue
		}

		environPath := filepath.Join("/proc", pid, "environ")
		data, err := os.ReadFile(environPath)
		if err != nil {
			continue
		}

		if strings.Contains(string(data), marker) {
			// This process belongs to our prefix — kill it
			cmdline, _ := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
			if strings.Contains(string(cmdline), ".exe") {
				pidNum := 0
				for _, c := range pid {
					pidNum = pidNum*10 + int(c-'0')
				}
				proc, err := os.FindProcess(pidNum)
				if err == nil {
					_ = proc.Signal(syscall.SIGKILL)
				}
			}
		}
	}
}

// buildProtonEnv builds the full environment for running the proton script.
func (a *Adapter) buildProtonEnv(prefixPath string, protonEnv *domain.ProtonEnv) map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	env["STEAM_COMPAT_DATA_PATH"] = prefixPath
	env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = "/usr/share/steam"
	env["SteamGameId"] = "0"
	env["SteamAppId"] = "0"
	env["STORE"] = "battlenet"

	if protonEnv != nil {
		for k, v := range protonEnv.Vars {
			env[k] = v
		}
	}

	return env
}

// buildBaseEnv creates the base environment for direct wine/wineserver operations.
func (a *Adapter) buildBaseEnv(runtime *domain.ProtonRuntime, prefixPath string) map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	env["WINEPREFIX"] = filepath.Join(prefixPath, "pfx")
	env["STEAM_COMPAT_DATA_PATH"] = prefixPath
	env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = "/usr/share/steam"

	ldPaths := []string{
		runtime.LibDir + "x86_64-linux-gnu",
		runtime.LibDir + "i386-linux-gnu",
	}
	if existing, ok := env["LD_LIBRARY_PATH"]; ok && existing != "" {
		env["LD_LIBRARY_PATH"] = strings.Join(ldPaths, ":") + ":" + existing
	} else {
		env["LD_LIBRARY_PATH"] = strings.Join(ldPaths, ":")
	}

	if existing, ok := env["PATH"]; ok {
		env["PATH"] = runtime.BinDir + ":" + existing
	} else {
		env["PATH"] = runtime.BinDir
	}

	return env
}

func envMapToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
