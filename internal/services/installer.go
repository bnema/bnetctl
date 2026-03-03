package services

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// InstallStatus represents the current phase of installation
type InstallStatus int

const (
	InstallDownloading InstallStatus = iota
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
	Download *ports.DownloadProgress
}

// InstallerService orchestrates the Battle.net installation process
type InstallerService struct {
	runtime    ports.RuntimePort
	downloader ports.DownloaderPort
	fs         ports.FilesystemPort
	cfg        *domain.Config
}

// NewInstallerService creates a new installer service
func NewInstallerService(
	runtime ports.RuntimePort,
	downloader ports.DownloaderPort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
) *InstallerService {
	return &InstallerService{
		runtime:    runtime,
		downloader: downloader,
		fs:         fs,
		cfg:        cfg,
	}
}

// Install performs the full installation with progress reporting:
// 1. Detect proton-cachyos
// 2. Create directories
// 3. Download Battle.net-Setup.exe
// 4. Create Wine prefix
// 5. Run the installer asynchronously
// 6. Poll until Battle.net.exe appears in prefix
// 7. Kill wineserver to clean up
func (s *InstallerService) Install(progressFn func(InstallProgress)) (*domain.Installation, error) {
	report := func(status InstallStatus, msg string) {
		if progressFn != nil {
			progressFn(InstallProgress{Status: status, Message: msg})
		}
	}

	// Step 1: Detect runtime
	if _, err := s.runtime.Detect(); err != nil {
		return nil, fmt.Errorf("detect proton: %w", err)
	}

	// Step 2: Ensure directories
	if err := s.fs.EnsureDirs(s.cfg); err != nil {
		return nil, fmt.Errorf("create directories: %w", err)
	}

	// Step 3: Download installer
	setupPath := filepath.Join(s.cfg.CacheDir, "Battle.net-Setup.exe")
	if !s.fs.Exists(setupPath) {
		report(InstallDownloading, "Downloading Battle.net installer...")
		dlProgressFn := func(p ports.DownloadProgress) {
			if progressFn != nil {
				progressFn(InstallProgress{
					Status:   InstallDownloading,
					Message:  "Downloading Battle.net installer...",
					Download: &p,
				})
			}
		}
		if err := s.downloader.Download(domain.BattleNetSetupURL, setupPath, dlProgressFn); err != nil {
			return nil, fmt.Errorf("download Battle.net installer: %w", err)
		}
	}

	// Step 4: Create prefix if needed
	pfxDir := filepath.Join(s.cfg.PrefixDir, "pfx")
	if !s.fs.Exists(pfxDir) {
		report(InstallCreatingPrefix, "Creating Wine prefix...")
		if err := s.runtime.CreatePrefix(s.cfg.PrefixDir); err != nil {
			return nil, fmt.Errorf("create Wine prefix: %w", err)
		}
	}

	// Step 5: Launch installer asynchronously
	report(InstallRunningSetup, "Running Battle.net installer...")
	done, err := s.runtime.RunExeAsync(domain.VerbRunInPrefix, s.cfg.PrefixDir, setupPath, nil)
	if err != nil {
		return nil, fmt.Errorf("start Battle.net installer: %w", err)
	}

	// Step 6: Poll until Battle.net.exe appears or installer process exits
	exePath := filepath.Join(s.cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	report(InstallWaitingForClient, "Waiting for Battle.net to install (this may take a few minutes)...")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case exitErr := <-done:
			// Installer process exited
			if exitErr != nil {
				// Check if Battle.net got installed anyway (installer might exit non-zero
				// after launching the client)
				if !s.fs.Exists(exePath) {
					return nil, fmt.Errorf("Battle.net installer failed: %w", exitErr)
				}
			}
			// Process exited — kill wineserver and wait for it to fully terminate
			report(InstallDone, "Cleaning up...")
			_ = s.runtime.KillPrefix(s.cfg.PrefixDir)
			_ = s.runtime.WaitPrefix(s.cfg.PrefixDir)
			return s.buildResult(exePath, setupPath), nil

		case <-ticker.C:
			if s.fs.Exists(exePath) {
				// Battle.net.exe found — installation succeeded
				report(InstallDone, "Battle.net client detected, cleaning up...")
				_ = s.runtime.KillPrefix(s.cfg.PrefixDir)
				_ = s.runtime.WaitPrefix(s.cfg.PrefixDir)
				return s.buildResult(exePath, setupPath), nil
			}
			// Still waiting, keep polling
			report(InstallWaitingForClient, "Waiting for Battle.net to install...")
		}
	}
}

// GetInstallation returns the current installation state
func (s *InstallerService) GetInstallation() *domain.Installation {
	exePath := filepath.Join(s.cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	return &domain.Installation{
		PrefixPath: s.cfg.PrefixDir,
		ExePath:    exePath,
		SetupPath:  filepath.Join(s.cfg.CacheDir, "Battle.net-Setup.exe"),
		Installed:  s.fs.Exists(exePath),
	}
}

func (s *InstallerService) buildResult(exePath, setupPath string) *domain.Installation {
	return &domain.Installation{
		PrefixPath: s.cfg.PrefixDir,
		ExePath:    exePath,
		SetupPath:  setupPath,
		Installed:  s.fs.Exists(exePath),
	}
}
