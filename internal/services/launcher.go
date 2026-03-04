package services

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// LaunchService orchestrates launching Battle.net
type LaunchService struct {
	runtime ports.RuntimePort
	fs      ports.FilesystemPort
	cfg     *domain.Config
	envFn   func() *domain.ProtonEnv
}

// NewLaunchService creates a new launch service.
// envFn is a function that returns GPU/system-specific environment variables.
func NewLaunchService(
	runtime ports.RuntimePort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
	envFn func() *domain.ProtonEnv,
) *LaunchService {
	return &LaunchService{
		runtime: runtime,
		fs:      fs,
		cfg:     cfg,
		envFn:   envFn,
	}
}

// Launch starts Battle.net via Proton.
func (s *LaunchService) Launch() error {
	// Check proton is available
	runtime, err := s.runtime.Detect()
	if err != nil {
		return fmt.Errorf("detect proton: %w", err)
	}
	_ = runtime

	// Check Battle.net is installed
	inst := s.getInstallation()
	if !inst.Installed {
		return fmt.Errorf("Battle.net is not installed. Run 'bnetctl install' first")
	}

	// Check if already running
	if s.runtime.IsProcessRunning(s.cfg.PrefixDir) {
		return fmt.Errorf("Battle.net is already running")
	}

	// Build environment
	var env *domain.ProtonEnv
	if s.envFn != nil {
		env = s.envFn()
	}
	if err := EnsureBattleNetConfig(s.cfg.PrefixDir); err != nil {
		return fmt.Errorf("ensure Battle.net config: %w", err)
	}

	// Launch proton as async so we get access to the process for cleanup
	done, err := s.runtime.RunExeAsync(domain.VerbRun, s.cfg.PrefixDir, inst.ExePath, env)
	if err != nil {
		return fmt.Errorf("launch Battle.net: %w", err)
	}

	// Trap signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Wait for either proton to exit or a signal
	var runErr error
	select {
	case runErr = <-done:
		// Proton exited on its own
	case <-sigCh:
		// Signal received — cleanup below
	}

	// Best-effort cleanup: kill wineserver + orphaned processes
	fmt.Println("\nShutting down Battle.net...")
	_ = s.runtime.GracefulKillPrefix(s.cfg.PrefixDir, 5*time.Second)

	return runErr
}

func (s *LaunchService) getInstallation() *domain.Installation {
	exePath := s.cfg.PrefixDir + "/pfx/" + domain.BattleNetExeRelPath
	return &domain.Installation{
		PrefixPath: s.cfg.PrefixDir,
		ExePath:    exePath,
		Installed:  s.fs.Exists(exePath),
	}
}
