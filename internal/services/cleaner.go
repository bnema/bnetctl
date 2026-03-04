package services

import (
	"fmt"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/logger"
	"github.com/bnema/bnetctl/internal/ports"
)

// CleanerService handles cleanup of bnetctl data
type CleanerService struct {
	runtime ports.RuntimePort
	desktop ports.DesktopPort
	fs      ports.FilesystemPort
	cfg     *domain.Config
}

// NewCleanerService creates a new cleaner service
func NewCleanerService(
	runtime ports.RuntimePort,
	desktop ports.DesktopPort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
) *CleanerService {
	return &CleanerService{
		runtime: runtime,
		desktop: desktop,
		fs:      fs,
		cfg:     cfg,
	}
}

// CleanCache removes cached files (downloaded installer, logs)
func (s *CleanerService) CleanCache() error {
	log := logger.Log
	log.Debug("cleaning cache", "path", s.cfg.CacheDir)

	if s.fs.Exists(s.cfg.CacheDir) {
		if err := s.fs.Remove(s.cfg.CacheDir); err != nil {
			return fmt.Errorf("remove cache: %w", err)
		}
	}
	return nil
}

// CleanPrefix removes the Wine prefix (kills running processes first)
func (s *CleanerService) CleanPrefix() error {
	log := logger.Log
	log.Debug("cleaning prefix", "path", s.cfg.PrefixDir)

	// Kill any running Wine processes
	if s.runtime.IsProcessRunning(s.cfg.PrefixDir) {
		log.Debug("killing running wine processes before prefix removal")
		if err := s.runtime.KillPrefix(s.cfg.PrefixDir); err != nil {
			return fmt.Errorf("kill wine processes: %w", err)
		}
	}

	if s.fs.Exists(s.cfg.PrefixDir) {
		if err := s.fs.Remove(s.cfg.PrefixDir); err != nil {
			return fmt.Errorf("remove prefix: %w", err)
		}
	}
	return nil
}

// CleanAll removes everything: cache, prefix, desktop entry, data dir
func (s *CleanerService) CleanAll() error {
	log := logger.Log
	log.Info("cleaning all bnetctl data")

	if err := s.CleanPrefix(); err != nil {
		return err
	}

	if err := s.CleanCache(); err != nil {
		return err
	}

	// Remove desktop entry
	_ = s.desktop.RemoveEntry("bnetctl")

	// Remove data dir
	if s.fs.Exists(s.cfg.DataDir) {
		if err := s.fs.Remove(s.cfg.DataDir); err != nil {
			return fmt.Errorf("remove data dir: %w", err)
		}
	}

	return nil
}
