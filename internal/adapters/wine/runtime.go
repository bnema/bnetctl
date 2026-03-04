package wine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// Adapter implements ports.RuntimePort using wine-cachyos directly
type Adapter struct {
	log *log.Logger
}

var _ ports.RuntimePort = (*Adapter)(nil)

// NewAdapter creates a new Wine adapter
func NewAdapter(log *log.Logger) *Adapter {
	return &Adapter{log: log}
}

// Detect checks if wine-cachyos is available and returns runtime info
func (a *Adapter) Detect() (*domain.WineRuntime, error) {
	log := a.log

	wineBin, err := exec.LookPath("wine")
	if err != nil {
		return nil, fmt.Errorf("wine not found: %w (install wine-cachyos: paru -S wine-cachyos)", err)
	}

	wineBootBin, err := exec.LookPath("wineboot")
	if err != nil {
		return nil, fmt.Errorf("wineboot not found: %w", err)
	}

	wineServerBin, err := exec.LookPath("wineserver")
	if err != nil {
		return nil, fmt.Errorf("wineserver not found: %w", err)
	}
	log.Debug("detecting wine", "wine", wineBin, "wineboot", wineBootBin, "wineserver", wineServerBin)

	// Get version
	out, err := exec.Command(wineBin, "--version").Output()
	if err != nil {
		return nil, fmt.Errorf("get wine version: %w", err)
	}
	version := strings.TrimSpace(string(out))
	log.Debug("wine version", "version", version)

	// Check NTSync availability
	hasNTSync := false
	if _, err := os.Stat("/dev/ntsync"); err == nil {
		hasNTSync = true
	}
	log.Debug("ntsync", "available", hasNTSync)

	// Check DXVK setup availability
	dxvkSetupBin, _ := exec.LookPath("setup_dxvk")
	log.Debug("dxvk setup", "available", dxvkSetupBin != "", "path", dxvkSetupBin)

	// Check VKD3D-proton setup availability
	vkd3dSetupBin, _ := exec.LookPath("setup_vkd3d_proton")

	return &domain.WineRuntime{
		WineBin:       wineBin,
		WineBootBin:   wineBootBin,
		WineServerBin: wineServerBin,
		Version:       version,
		HasNTSync:     hasNTSync,
		HasDXVKSetup:  dxvkSetupBin != "",
		DXVKSetupBin:  dxvkSetupBin,
		HasVKD3DSetup: vkd3dSetupBin != "",
		VKD3DSetupBin: vkd3dSetupBin,
	}, nil
}

// CreatePrefix initializes a new Wine prefix at the given path
func (a *Adapter) CreatePrefix(prefixPath string) error {
	log := a.log

	rt, err := a.Detect()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Info("creating wine prefix", "path", pfxDir)
	if err := os.MkdirAll(pfxDir, 0o755); err != nil {
		return fmt.Errorf("create prefix directory: %w", err)
	}

	env := a.buildEnv(pfxDir, nil)
	env["WINEDEBUG"] = "-all"

	// Initialize prefix with wineboot
	log.Debug("running wineboot --init")
	cmd := exec.Command(rt.WineBootBin, "--init")
	cmd.Env = envMapToSlice(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wineboot --init failed: %w", err)
	}

	// Wait for wineserver to finish
	log.Debug("waiting for wineserver")
	waitCmd := exec.Command(rt.WineServerBin, "-w")
	waitCmd.Env = envMapToSlice(env)
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("wineserver -w failed: %w", err)
	}

	// Auto-install DXVK if setup_dxvk is available
	log.Debug("dxvk auto-install", "enabled", rt.HasDXVKSetup)
	if rt.HasDXVKSetup {
		dxvkCmd := exec.Command(rt.DXVKSetupBin, "install", "--symlink")
		dxvkCmd.Env = envMapToSlice(env)
		dxvkCmd.Stdout = os.Stdout
		dxvkCmd.Stderr = os.Stderr
		if err := dxvkCmd.Run(); err != nil {
			// DXVK install failure is non-fatal
			log.Warn("dxvk setup failed", "error", err)
			fmt.Fprintf(os.Stderr, "warning: DXVK setup failed (non-fatal): %v\n", err)
		} else {
			log.Info("dxvk installed")
		}
	}

	// Auto-install VKD3D-proton if setup_vkd3d_proton is available
	if rt.HasVKD3DSetup {
		log.Info("installing vkd3d-proton", "path", rt.VKD3DSetupBin)
		vkd3dCmd := exec.Command(rt.VKD3DSetupBin, "install", "--symlink")
		vkd3dCmd.Env = envMapToSlice(env)
		vkd3dCmd.Stdout = os.Stdout
		vkd3dCmd.Stderr = os.Stderr
		if err := vkd3dCmd.Run(); err != nil {
			// VKD3D install failure is non-fatal
			log.Warn("vkd3d-proton setup failed (non-fatal)", "error", err)
		}
	}

	// Disable Wine systray icon (floats as orphan window on Wayland tiling WMs)
	log.Debug("disabling wine systray")
	regCmd := exec.Command(rt.WineBin, "reg", "add",
		`HKCU\Software\Wine\Explorer`, "/v", "ShowSystray",
		"/t", "REG_DWORD", "/d", "0", "/f")
	regCmd.Env = envMapToSlice(env)
	regCmd.Stdout = os.Stdout
	regCmd.Stderr = os.Stderr
	if err := regCmd.Run(); err != nil {
		log.Warn("failed to disable systray (non-fatal)", "error", err)
	}
	// Wait for wineserver after reg edit
	waitRegCmd := exec.Command(rt.WineServerBin, "-w")
	waitRegCmd.Env = envMapToSlice(env)
	_ = waitRegCmd.Run()

	return nil
}

// RunExe runs a Windows executable inside the prefix using wine (blocking)
func (a *Adapter) RunExe(prefixPath string, exePath string, wineEnv *domain.WineEnv) error {
	log := a.log

	rt, err := a.Detect()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("running exe", "wine", rt.WineBin, "exe", exePath, "prefix", pfxDir)
	env := a.buildEnv(pfxDir, wineEnv)

	cmd := exec.Command(rt.WineBin, exePath)
	cmd.Env = envMapToSlice(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunExeAsync runs a Windows executable inside the prefix without blocking
func (a *Adapter) RunExeAsync(prefixPath string, exePath string, wineEnv *domain.WineEnv) (<-chan error, error) {
	log := a.log

	rt, err := a.Detect()
	if err != nil {
		return nil, err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("starting exe async", "wine", rt.WineBin, "exe", exePath, "prefix", pfxDir)
	env := a.buildEnv(pfxDir, wineEnv)

	cmd := exec.Command(rt.WineBin, exePath)
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
	log := a.log

	wineServerBin, err := exec.LookPath("wineserver")
	if err != nil {
		return false
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("checking wineserver status", "prefix", pfxDir)
	env := a.buildEnv(pfxDir, nil)

	cmd := exec.Command(wineServerBin, "-k0")
	cmd.Env = envMapToSlice(env)
	return cmd.Run() == nil
}

// KillPrefix stops all Wine processes in the given prefix
func (a *Adapter) KillPrefix(prefixPath string) error {
	log := a.log

	wineServerBin, err := exec.LookPath("wineserver")
	if err != nil {
		return fmt.Errorf("wineserver not found: %w", err)
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Info("killing wineserver", "prefix", pfxDir)
	env := a.buildEnv(pfxDir, nil)

	cmd := exec.Command(wineServerBin, "-k")
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

// WaitPrefix blocks until the wineserver for the given prefix exits
func (a *Adapter) WaitPrefix(prefixPath string) error {
	wineServerBin, err := exec.LookPath("wineserver")
	if err != nil {
		return fmt.Errorf("wineserver not found: %w", err)
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	env := a.buildEnv(pfxDir, nil)

	cmd := exec.Command(wineServerBin, "-w")
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

// GracefulKillPrefix attempts a graceful stop, waits up to timeout, then force kills.
// Also cleans up orphaned processes that survive wineserver shutdown.
func (a *Adapter) GracefulKillPrefix(prefixPath string, timeout time.Duration) error {
	log := a.log
	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("graceful kill initiated", "prefix", pfxDir, "timeout", timeout)

	_ = a.KillPrefix(prefixPath)

	done := make(chan error, 1)
	go func() {
		done <- a.WaitPrefix(prefixPath)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		log.Warn("graceful kill timed out, force killing")
		wineServerBin, _ := exec.LookPath("wineserver")
		if wineServerBin != "" {
			env := a.buildEnv(pfxDir, nil)
			cmd := exec.Command(wineServerBin, "-k9")
			cmd.Env = envMapToSlice(env)
			_ = cmd.Run()
		}
	}

	_, _ = a.KillOrphans(prefixPath)
	log.Debug("orphan cleanup complete")
	return nil
}

// KillOrphans finds and kills any processes whose environment contains WINEPREFIX
// matching our prefix. Returns the PIDs of killed processes.
func (a *Adapter) KillOrphans(prefixPath string) ([]int, error) {
	log := a.log

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	marker := "WINEPREFIX=" + pfxDir
	myPid := os.Getpid()
	var killed []int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if pid[0] < '1' || pid[0] > '9' {
			continue
		}

		pidNum := 0
		for _, c := range pid {
			pidNum = pidNum*10 + int(c-'0')
		}
		if pidNum == myPid {
			continue
		}

		environPath := filepath.Join("/proc", pid, "environ")
		data, err := os.ReadFile(environPath)
		if err != nil {
			continue
		}

		if strings.Contains(string(data), marker) {
			proc, err := os.FindProcess(pidNum)
			if err == nil {
				log.Debug("killing orphaned process", "pid", pidNum)
				if sigErr := proc.Signal(syscall.SIGKILL); sigErr == nil {
					killed = append(killed, pidNum)
				}
			}
		}
	}

	return killed, nil
}

// buildEnv builds the environment for Wine commands.
func (a *Adapter) buildEnv(pfxDir string, wineEnv *domain.WineEnv) map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	env["WINEPREFIX"] = pfxDir

	if wineEnv != nil {
		for k, v := range wineEnv.Vars {
			env[k] = v
		}
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
