package proton

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

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
	argv := []string{protonScript, verb, exePath}

	// Use syscall.Exec to replace current process for "run" verb (launch)
	if verb == domain.VerbRun || verb == domain.VerbWaitForExitAndRun {
		python, err := exec.LookPath("python3")
		if err != nil {
			return fmt.Errorf("python3 not found: %w", err)
		}
		fullArgv := append([]string{python}, argv...)
		return syscall.Exec(python, fullArgv, envMapToSlice(env))
	}

	cmd := exec.Command("python3", argv...)
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

// IsProcessRunning checks if a Wine process is running in the given prefix
func (a *Adapter) IsProcessRunning(prefixPath string) bool {
	runtime, err := a.Detect()
	if err != nil {
		return false
	}

	env := a.buildBaseEnv(runtime, prefixPath)
	wineserverBin := filepath.Join(runtime.BinDir, "wineserver")

	cmd := exec.Command(wineserverBin, "-p")
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
