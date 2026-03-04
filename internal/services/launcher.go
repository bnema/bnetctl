package services

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/logger"
	"github.com/bnema/bnetctl/internal/ports"
)

// LaunchService orchestrates launching Battle.net
type LaunchService struct {
	runtime ports.RuntimePort
	fs      ports.FilesystemPort
	cfg     *domain.Config
	envFn   func() *domain.WineEnv
}

// NewLaunchService creates a new launch service.
// envFn is a function that returns GPU/system-specific environment variables.
func NewLaunchService(
	runtime ports.RuntimePort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
	envFn func() *domain.WineEnv,
) *LaunchService {
	return &LaunchService{
		runtime: runtime,
		fs:      fs,
		cfg:     cfg,
		envFn:   envFn,
	}
}

// Launch starts Battle.net via Wine.
func (s *LaunchService) Launch() error {
	log := logger.Log

	// Check wine is available
	log.Debug("checking wine runtime")
	runtime, err := s.runtime.Detect()
	if err != nil {
		log.Error("wine runtime detection failed", "error", err)
		return fmt.Errorf("detect wine: %w", err)
	}
	_ = runtime

	// Check Battle.net is installed
	inst := s.getInstallation()
	log.Debug("checking installation", "exe", inst.ExePath, "installed", inst.Installed)
	if !inst.Installed {
		return fmt.Errorf("battle.net is not installed. Run 'bnetctl install' first")
	}

	// Check if already running
	log.Debug("checking if already running")
	if s.runtime.IsProcessRunning(s.cfg.PrefixDir) {
		return fmt.Errorf("battle.net is already running")
	}

	// Build environment
	log.Debug("building gpu environment")
	var env *domain.WineEnv
	if s.envFn != nil {
		env = s.envFn()
	}
	log.Debug("ensuring battle.net config")
	if err := EnsureBattleNetConfig(s.cfg.PrefixDir, s.fs); err != nil {
		log.Error("ensure battle.net config failed", "error", err)
		return fmt.Errorf("ensure Battle.net config: %w", err)
	}

	// Launch wine async so we get access to the process for cleanup
	log.Info("launching battle.net async", "exe", inst.ExePath)
	done, err := s.runtime.RunExeAsync(s.cfg.PrefixDir, inst.ExePath, env)
	if err != nil {
		log.Error("launch battle.net failed", "error", err)
		return fmt.Errorf("launch Battle.net: %w", err)
	}

	// Trap signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Wait for either wine to exit or a signal
	var runErr error
	select {
	case runErr = <-done:
		log.Info("wine process exited", "error", runErr)
		// Wine exited on its own
	case <-sigCh:
		log.Info("received signal, shutting down")
		// Signal received — cleanup below
	}

	// Best-effort cleanup: kill wineserver + orphaned processes
	fmt.Println("\nShutting down Battle.net...")
	log.Debug("graceful kill initiated")
	_ = s.runtime.GracefulKillPrefix(s.cfg.PrefixDir, 5*time.Second)

	return runErr
}

func (s *LaunchService) getInstallation() *domain.Installation {
	exePath := filepath.Join(s.cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	return &domain.Installation{
		PrefixPath: s.cfg.PrefixDir,
		ExePath:    exePath,
		Installed:  s.fs.Exists(exePath),
	}
}
