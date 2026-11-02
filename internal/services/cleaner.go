package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// CleanerService handles cleanup of bnetctl data
type CleanerService struct {
	runtime ports.RuntimePort
	desktop ports.DesktopPort
	fs      ports.FilesystemPort
	cfg     *domain.Config
	log     *log.Logger
}

// NewCleanerService creates a new cleaner service
func NewCleanerService(
	runtime ports.RuntimePort,
	desktop ports.DesktopPort,
	fs ports.FilesystemPort,
	cfg *domain.Config,
	log *log.Logger,
) *CleanerService {
	return &CleanerService{
		runtime: runtime,
		desktop: desktop,
		fs:      fs,
		cfg:     cfg,
		log:     log,
	}
}

// CleanCache removes cached files (downloaded installer, logs)
func (s *CleanerService) CleanCache() error {
	log := s.log
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
	log := s.log
	log.Debug("cleaning prefix", "path", s.cfg.PrefixDir)

	// A prefix whose wineserver is gone still holds wine processes, and removing
	// the directory under them leaves them running against deleted files.
	return s.runtime.WithStoppedPrefix(s.cfg.PrefixDir, 10*time.Second, func() error {
		if s.fs.Exists(s.cfg.PrefixDir) {
			if err := s.fs.Remove(s.cfg.PrefixDir); err != nil {
				return fmt.Errorf("remove prefix: %w", err)
			}
		}
		return nil
	})
}

// CleanAll removes everything: cache, prefix, desktop entry, data dir
func (s *CleanerService) CleanAll() error {
	log := s.log
	log.Info("cleaning all bnetctl data")

	return s.runtime.WithStoppedPrefix(s.cfg.PrefixDir, 10*time.Second, func() error {
		if err := s.CleanCache(); err != nil {
			return err
		}
		for _, name := range []string{ports.EntryWayland, ports.EntryX11, ports.EntryLegacy} {
			_ = s.desktop.RemoveEntry(name)
		}
		// Preserve the persistent lock inode and every ancestor containing it.
		return s.removeContentsExcept(s.cfg.DataDir, filepath.Clean(s.cfg.PrefixDir)+".lock")
	})
}

func (s *CleanerService) removeContentsExcept(dir, keep string) error {
	entries, err := s.fs.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read data directory: %w", err)
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if path == keep {
			continue
		}
		if strings.HasPrefix(keep, path+string(filepath.Separator)) {
			if err := s.removeContentsExcept(path, keep); err != nil {
				return err
			}
		} else if err := s.fs.Remove(path); err != nil {
			return fmt.Errorf("remove data: %w", err)
		}
	}
	return nil
}
