package services

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// InstallStatus represents the current phase of installation
type InstallStatus int

const (
	InstallDetecting InstallStatus = iota
	InstallDownloading
	InstallCreatingPrefix
	InstallRunningSetup
	InstallWaitingForClient
	InstallDone
	InstallFailed
)

// InstallProgress reports installation progress to the caller
type InstallProgress struct {
	Status  InstallStatus
	Message string
	// Download is non-nil during the download phase
	Download *domain.DownloadProgress
	// Runtime is non-nil on first progress event after wine detection
	Runtime *domain.WineRuntime
}

// InstallerService orchestrates the Battle.net installation process
type InstallerService struct {
	runtime    ports.RuntimePort
	downloader ports.DownloaderPort
	fs         ports.FilesystemPort
	cfg        *domain.Config
	envFn      func() *domain.WineEnv
	exeArgs    []string
	log        *log.Logger
}

// NewInstallerService creates a new installer service
func NewInstallerService(
	runtime ports.RuntimePort,
	downloader ports.DownloaderPort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
	envFn func() *domain.WineEnv,
	log *log.Logger,
	exeArgs ...string,
) *InstallerService {
	return &InstallerService{
		runtime:    runtime,
		downloader: downloader,
		fs:         fs,
		cfg:        cfg,
		envFn:      envFn,
		exeArgs:    exeArgs,
		log:        log,
	}
}

// Install performs the full installation with progress reporting:
// 1. Detect wine-cachyos
// 2. Create directories
// 3. Download Battle.net-Setup.exe
// 4. Create Wine prefix
// 5. Run the installer asynchronously
// 6. Wait for installer to exit gracefully
// 7. Clean up wineserver
func (s *InstallerService) Install(progressFn func(InstallProgress)) (*domain.Installation, error) {
	log := s.log

	report := func(status InstallStatus, msg string) {
		if progressFn != nil {
			progressFn(InstallProgress{Status: status, Message: msg})
		}
	}

	// Step 1: Detect runtime
	log.Debug("detecting wine runtime")
	rt, err := s.runtime.Detect()
	if err != nil {
		log.Error("wine runtime detection failed", "error", err)
		return nil, fmt.Errorf("detect wine: %w", err)
	}
	if progressFn != nil {
		progressFn(InstallProgress{Status: InstallDetecting, Message: "Wine detected", Runtime: rt})
	}

	// Step 2: Ensure directories
	log.Debug("ensuring directories", "data", s.cfg.DataDir, "cache", s.cfg.CacheDir, "prefix", s.cfg.PrefixDir)
	if err := s.fs.EnsureDirs(s.cfg); err != nil {
		log.Error("ensure directories failed", "error", err)
		return nil, fmt.Errorf("create directories: %w", err)
	}

	// Step 3: Download installer
	setupPath := filepath.Join(s.cfg.CacheDir, "Battle.net-Setup.exe")
	if !s.fs.Exists(setupPath) {
		log.Debug("downloading installer", "url", domain.BattleNetSetupURL, "dest", setupPath)
		report(InstallDownloading, "Downloading Battle.net installer...")
		dlProgressFn := func(p domain.DownloadProgress) {
			if progressFn != nil {
				progressFn(InstallProgress{
					Status:   InstallDownloading,
					Message:  "Downloading Battle.net installer...",
					Download: &p,
				})
			}
		}
		if err := s.downloader.Download(domain.BattleNetSetupURL, setupPath, dlProgressFn); err != nil {
			log.Error("downloading installer failed", "error", err)
			return nil, fmt.Errorf("download Battle.net installer: %w", err)
		}
	}

	// Step 4: Create prefix if needed
	pfxDir := filepath.Join(s.cfg.PrefixDir, "pfx")
	if !s.fs.Exists(pfxDir) {
		log.Debug("creating wine prefix", "path", s.cfg.PrefixDir)
		report(InstallCreatingPrefix, "Creating Wine prefix...")
		if err := s.runtime.CreatePrefix(s.cfg.PrefixDir); err != nil {
			log.Error("create wine prefix failed", "error", err)
			return nil, fmt.Errorf("create Wine prefix: %w", err)
		}
	}

	// Step 5: Launch installer asynchronously
	log.Debug("launching installer async", "setup", setupPath, "args", s.exeArgs)
	report(InstallRunningSetup, "Running Battle.net installer...")
	var wineEnv *domain.WineEnv
	if s.envFn != nil {
		wineEnv = s.envFn()
	}
	done, err := s.runtime.RunExeAsync(s.cfg.PrefixDir, setupPath, wineEnv, s.exeArgs...)
	if err != nil {
		log.Error("start installer failed", "error", err)
		return nil, fmt.Errorf("start Battle.net installer: %w", err)
	}

	// Step 6: Poll until Battle.net.exe appears or installer process exits
	exePath := filepath.Join(s.cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	report(InstallWaitingForClient, "Waiting for Battle.net to install (this may take a few minutes)...")

	timeout := time.NewTimer(30 * time.Minute)
	defer timeout.Stop()
	log.Debug("waiting for installer to exit", "timeout", "30m")

	select {
	case exitErr := <-done:
		log.Info("installer process exited", "error", exitErr)
		if exitErr != nil && !s.fs.Exists(exePath) {
			return nil, fmt.Errorf("battle.net installer failed: %w", exitErr)
		}
	case <-timeout.C:
		log.Error("installer timed out")
		_ = s.runtime.GracefulKillPrefix(s.cfg.PrefixDir, 10*time.Second)
		return nil, fmt.Errorf("installation timed out after 30 minutes")
	}

	// Installer exited — give Battle.net Agent a moment to finish, then clean up
	report(InstallDone, "Installer finished, waiting for Battle.net Agent to settle...")
	time.Sleep(10 * time.Second)
	log.Debug("post-install settle complete, cleaning up")

	_ = s.runtime.GracefulKillPrefix(s.cfg.PrefixDir, 10*time.Second)

	if err := EnsureBattleNetConfig(s.cfg.PrefixDir, s.fs, s.log); err != nil {
		log.Error("ensure battle.net config failed", "error", err)
		return nil, fmt.Errorf("ensure Battle.net config: %w", err)
	}
	log.Info("battle.net config ensured", "prefix", s.cfg.PrefixDir)

	return s.buildResult(exePath, setupPath), nil
}

// IsInstalled reports whether the Battle.net client exists in the prefix
func IsInstalled(fs ports.FilesystemPort, cfg *domain.Config) bool {
	return fs.Exists(filepath.Join(cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath))
}

// buildResult reports the installation state left behind by a completed setup run
func (s *InstallerService) buildResult(exePath, setupPath string) *domain.Installation {
	return &domain.Installation{
		PrefixPath: s.cfg.PrefixDir,
		ExePath:    exePath,
		SetupPath:  setupPath,
		Installed:  IsInstalled(s.fs, s.cfg),
	}
}
