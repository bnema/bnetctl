package wine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

const (
	wineOverrideEnv   = "BNETCTL_WINE"
	cachyOSWineBinary = "/opt/wine-cachyos/bin/wine"
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

	wineBin, wineBootBin, wineServerBin, err := resolveBinaries()
	if err != nil {
		return nil, err
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

	// Disable Wine's systray window (tiled as a full-size empty column on
	// Wayland compositors) before the prefix is used.
	log.Debug("disabling wine systray")
	if err := a.DisableSystray(prefixPath); err != nil {
		log.Warn("failed to disable systray (non-fatal)", "error", err)
	}
	// Wait for wineserver after reg edit
	waitRegCmd := exec.Command(rt.WineServerBin, "-w")
	waitRegCmd.Env = envMapToSlice(env)
	_ = waitRegCmd.Run()

	return nil
}

// DisableSystray turns off Wine's standalone systray window for a prefix.
//
// Wine shows that window as soon as an application registers a tray icon
// (programs/explorer/desktop.c reads HKCU\Software\Wine\Explorer\ShowSystray as
// REG_DWORD). On tiling compositors it is tiled as a full-size empty column, so
// it must be disabled before the application starts - not only when the prefix
// is first created.
func (a *Adapter) DisableSystray(prefixPath string) error {
	rt, err := a.Detect()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	env := a.buildEnv(pfxDir, nil)
	env["WINEDEBUG"] = "-all"

	cmd := exec.Command(rt.WineBin, "reg", "add",
		`HKCU\Software\Wine\Explorer`, "/v", "ShowSystray",
		"/t", "REG_DWORD", "/d", "0", "/f")
	cmd.Env = envMapToSlice(env)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("disable wine systray: %w (%s)", err, strings.TrimSpace(string(out)))
	}
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
func (a *Adapter) RunExeAsync(prefixPath string, exePath string, wineEnv *domain.WineEnv, args ...string) (<-chan error, error) {
	log := a.log

	rt, err := a.Detect()
	if err != nil {
		return nil, err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("starting exe async", "wine", rt.WineBin, "exe", exePath, "args", args, "prefix", pfxDir)
	env := a.buildEnv(pfxDir, wineEnv)

	cmdArgs := append([]string{exePath}, args...)
	cmd := exec.Command(rt.WineBin, cmdArgs...)
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

	_, _, wineServerBin, err := resolveBinaries()
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

	_, _, wineServerBin, err := resolveBinaries()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Info("killing wineserver", "prefix", pfxDir)
	env := a.buildEnv(pfxDir, nil)

	cmd := exec.Command(wineServerBin, "-k")
	cmd.Env = envMapToSlice(env)
	return cmd.Run()
}

// battleNetProcessName is the executable that taskkill targets.
const battleNetProcessName = "Battle.net.exe"

// battleNetCloseWait is how long Battle.net is given to persist its session after
// it has been asked to close. A variable so tests can shorten it.
var battleNetCloseWait = 5 * time.Second

const (
	// prefixExitWait bounds the check that the prefix is really empty after
	// SIGKILL. Processes normally disappear at once, but a killed process can stay
	// visible for a moment.
	prefixExitWait = time.Second
	prefixExitPoll = 25 * time.Millisecond

	// battleNetClosePoll is the interval used while waiting for Battle.net to exit.
	battleNetClosePoll = 200 * time.Millisecond

	// battleNetCloseTimeout bounds the taskkill call, so a wineserver that stopped
	// answering cannot block a shutdown forever.
	battleNetCloseTimeout = 10 * time.Second
)

// closeBattleNetWindows asks Battle.net to close its own windows (WM_CLOSE)
// instead of killing it outright, so it can persist its session before the prefix
// is torn down. A killed launcher loses the session, and the next start then
// needs the interactive login popup, which does not complete under the native
// Wayland driver.
func (a *Adapter) closeBattleNetWindows(prefixPath string) {
	rt, err := a.Detect()
	if err != nil {
		a.log.Debug("cannot close Battle.net windows", "error", err)
		return
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	env := a.buildEnv(pfxDir, nil)
	env["WINEDEBUG"] = "-all"

	ctx, cancel := context.WithTimeout(context.Background(), battleNetCloseTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, rt.WineBin, "taskkill", "/IM", battleNetProcessName)
	cmd.Env = envMapToSlice(env)
	out, err := cmd.CombinedOutput()
	if err != nil {
		a.log.Debug("close Battle.net windows failed", "error", err, "output", strings.TrimSpace(string(out)))
		return
	}
	a.log.Debug("asked Battle.net to close", "output", strings.TrimSpace(string(out)))
}

// waitForBattleNetExit blocks until Battle.net is gone or max has elapsed.
// Polling instead of sleeping keeps a shutdown short when Battle.net already
// exited on its own.
func (a *Adapter) waitForBattleNetExit(prefixPath string, max time.Duration) {
	pfxDir := filepath.Join(prefixPath, "pfx")
	deadline := time.Now().Add(max)

	for {
		battleNet, err := prefixProcesses(pfxDir, battleNetProcessName)
		if err != nil || len(battleNet) == 0 {
			return
		}
		if !time.Now().Before(deadline) {
			a.log.Debug("Battle.net did not exit after the close request", "waited", max)
			return
		}
		time.Sleep(battleNetClosePoll)
	}
}

// GracefulKillPrefix stops everything running in the prefix and verifies the
// result. It returns an error if wine processes are still alive afterwards.
//
// A prefix without a wineserver is not a failure. Force killing a wineserver
// (the timeout path below) leaves its client processes behind, and those
// leftovers outlive every later attempt to stop the prefix unless they are
// reaped here - `wineserver -k` cannot reach them, because there is no server
// left to talk to.
func (a *Adapter) GracefulKillPrefix(prefixPath string, timeout time.Duration) error {
	log := a.log
	pfxDir := filepath.Join(prefixPath, "pfx")
	log.Debug("graceful kill initiated", "prefix", pfxDir, "timeout", timeout)

	// Let Battle.net save its session before the prefix is taken down.
	if a.IsProcessRunning(prefixPath) {
		a.closeBattleNetWindows(prefixPath)
		a.waitForBattleNetExit(prefixPath, battleNetCloseWait)
	}

	// Stop the wineserver. When none is running, the prefix only holds leftovers
	// from an earlier run, and the sweep below is what removes them.
	var forceErr error
	if err := a.KillPrefix(prefixPath); err != nil {
		log.Debug("wineserver not stopped", "prefix", pfxDir, "error", err)
	} else if err := a.waitForWineserver(prefixPath, timeout); err != nil {
		log.Warn("wineserver did not exit, force killing", "error", err)
		if forceErr = a.forceKillPrefix(prefixPath); forceErr != nil {
			log.Warn("force killing the wineserver failed", "error", forceErr)
		}
	}

	killed := a.KillOrphans(prefixPath)
	if len(killed) > 0 {
		log.Info("killed leftover wine processes", "pids", killed)
	}

	remaining, err := a.waitForPrefixExit(prefixPath)
	if err != nil {
		return errors.Join(forceErr, fmt.Errorf("verify that %s is stopped: %w", pfxDir, err))
	}
	if len(remaining) > 0 {
		err := fmt.Errorf("%d wine process(es) still running in %s: %v", len(remaining), pfxDir, remaining)
		return errors.Join(forceErr, err)
	}

	log.Debug("prefix stopped", "prefix", pfxDir)
	return nil
}

// waitForWineserver blocks until the prefix's wineserver exits or timeout elapses.
// The waiter is killed and reaped once the deadline passes, so it cannot outlive
// the call, and the wineserver binary is never confused with a leftover to sweep.
func (a *Adapter) waitForWineserver(prefixPath string, timeout time.Duration) error {
	_, _, wineServerBin, err := resolveBinaries()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	env := a.buildEnv(pfxDir, nil)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, wineServerBin, "-w")
	cmd.Env = envMapToSlice(env)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("wait for wineserver: timed out after %s", timeout)
		}
		return fmt.Errorf("wait for wineserver: %w", err)
	}
	return nil
}

// forceKillPrefix sends SIGKILL to the prefix's wineserver.
func (a *Adapter) forceKillPrefix(prefixPath string) error {
	_, _, wineServerBin, err := resolveBinaries()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(prefixPath, "pfx")
	env := a.buildEnv(pfxDir, nil)
	cmd := exec.Command(wineServerBin, "-k9")
	cmd.Env = envMapToSlice(env)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("force stop wineserver: %w", err)
	}
	return nil
}

// waitForPrefixExit returns the PIDs of prefix processes that survived SIGKILL.
// It waits briefly, because a killed process can stay visible for a moment.
func (a *Adapter) waitForPrefixExit(prefixPath string) ([]int, error) {
	pfxDir := filepath.Join(prefixPath, "pfx")
	deadline := time.Now().Add(prefixExitWait)

	for {
		remaining, err := prefixProcesses(pfxDir, "")
		if err != nil {
			return nil, err
		}
		if len(remaining) == 0 || !time.Now().Before(deadline) {
			return remaining, nil
		}
		time.Sleep(prefixExitPoll)
	}
}

// KillOrphans kills every process belonging to the prefix. Returns the PIDs of
// killed processes.
func (a *Adapter) KillOrphans(prefixPath string) []int {
	pfxDir := filepath.Join(prefixPath, "pfx")

	pids, err := prefixProcesses(pfxDir, "")
	if err != nil {
		a.log.Warn("cannot scan for leftover wine processes", "prefix", pfxDir, "error", err)
		return nil
	}

	var killed []int
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		a.log.Debug("killing orphaned process", "pid", pid)
		if err := proc.Signal(syscall.SIGKILL); err == nil {
			killed = append(killed, pid)
		}
	}

	return killed
}

// prefixProcesses returns the PIDs of live processes belonging to pfxDir, optionally
// restricted to processes named name (matched against /proc/<pid>/comm, which the
// kernel truncates to 15 bytes, so "Battle.net.exe" matches but longer names do
// not). Wine passes the Unix environment on to every Windows process it starts, so
// WINEPREFIX identifies a prefix's whole process tree, with or without a wineserver.
func prefixProcesses(pfxDir, name string) ([]int, error) {
	myPid := os.Getpid()
	var pids []int

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == myPid {
			continue
		}

		environ, comm, err := readProcess(entry.Name())
		if err != nil {
			continue
		}
		if !hasEnvEntry(environ, "WINEPREFIX="+pfxDir) || !isWineProcess(environ, comm) {
			continue
		}
		if name != "" && !strings.EqualFold(comm, name) {
			continue
		}

		pids = append(pids, pid)
	}

	return pids, nil
}

// readProcess returns the Unix environment and the process name of a PID.
func readProcess(pid string) (environ string, comm string, err error) {
	data, err := os.ReadFile(filepath.Join("/proc", pid, "environ"))
	if err != nil {
		return "", "", err
	}
	name, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
	if err != nil {
		return "", "", err
	}
	return string(data), strings.TrimSpace(string(name)), nil
}

// hasEnvEntry reports whether a /proc/<pid>/environ blob holds entry as a whole
// NUL-separated entry. Comparing whole entries keeps a prefix from matching its
// neighbours, for example "WINEPREFIX=/pfx-backup" against "/pfx".
func hasEnvEntry(environ, entry string) bool {
	for _, e := range strings.Split(environ, "\x00") {
		if e == entry {
			return true
		}
	}
	return false
}

// hasEnvKey reports whether a /proc/<pid>/environ blob sets key.
func hasEnvKey(environ, key string) bool {
	prefix := key + "="
	for _, e := range strings.Split(environ, "\x00") {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// isWineProcess reports whether a process belongs to a Wine session, rather than
// merely inheriting WINEPREFIX from the user's shell. Every Wine client carries
// Wine's own environment, and Wine names Windows processes after the executable it
// runs, which is enough to tell the two apart. Killing by WINEPREFIX alone would
// also hit terminals and editors started with WINEPREFIX exported.
func isWineProcess(environ, comm string) bool {
	if hasEnvKey(environ, "WINESERVERSOCKET") || hasEnvKey(environ, "WINELOADERNOEXEC") {
		return true
	}
	return strings.HasSuffix(comm, ".exe") || strings.HasPrefix(comm, "wine")
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
		for _, key := range wineEnv.Unset {
			delete(env, key)
		}
		for k, v := range wineEnv.Vars {
			env[k] = v
		}
	}

	return env
}

func resolveBinaries() (wine, wineboot, wineserver string, err error) {
	wine = os.Getenv(wineOverrideEnv)
	if wine == "" {
		if isExecutable(cachyOSWineBinary) {
			wine = cachyOSWineBinary
		} else if wine, err = exec.LookPath("wine"); err != nil {
			return "", "", "", fmt.Errorf("wine not found: %w (install wine-cachyos-opt or set %s)", err, wineOverrideEnv)
		}
	} else if !filepath.IsAbs(wine) {
		if wine, err = exec.LookPath(wine); err != nil {
			return "", "", "", fmt.Errorf("resolve %s: %w", wineOverrideEnv, err)
		}
	}
	if !isExecutable(wine) {
		return "", "", "", fmt.Errorf("wine executable is not usable: %s", wine)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wine); resolveErr == nil {
		wine = resolved
	}

	binDir := filepath.Dir(wine)
	wineboot = filepath.Join(binDir, "wineboot")
	wineserver = filepath.Join(binDir, "wineserver")
	if !isExecutable(wineboot) {
		return "", "", "", fmt.Errorf("wineboot not found next to wine: %s", wineboot)
	}
	if !isExecutable(wineserver) {
		return "", "", "", fmt.Errorf("wineserver not found next to wine: %s", wineserver)
	}
	return wine, wineboot, wineserver, nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0
}

func envMapToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
