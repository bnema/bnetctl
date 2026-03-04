package xdg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// Filesystem implements ports.FilesystemPort using standard OS operations with XDG paths
type Filesystem struct{}

var _ ports.FilesystemPort = (*Filesystem)(nil)

// NewFilesystem creates a new XDG filesystem adapter
func NewFilesystem() *Filesystem {
	return &Filesystem{}
}

// EnsureDirs creates all required directories
func (f *Filesystem) EnsureDirs(cfg *domain.Config) error {
	dirs := []string{
		cfg.DataDir,
		cfg.CacheDir,
		cfg.PrefixDir,
		cfg.GamesDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Exists checks if a path exists
func (f *Filesystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Remove removes a file or directory recursively
func (f *Filesystem) Remove(path string) error {
	return os.RemoveAll(path)
}

// HomeDir returns the user's home directory
func (f *Filesystem) HomeDir() (string, error) {
	return os.UserHomeDir()
}

func (f *Filesystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *Filesystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *Filesystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// LogFilePath returns the default log file path under XDG_CACHE_HOME.
func LogFilePath() (string, error) {
	cfg, err := DefaultConfig()
	if err != nil {
		return "", err
	}
	return cfg.LogFile, nil
}

// DefaultConfig returns a Config with XDG-compliant default paths
func DefaultConfig() (*domain.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dataDir := filepath.Join(home, ".local", "share", "bnetctl")
	cacheDir := filepath.Join(home, ".cache", "bnetctl")

	return &domain.Config{
		DataDir:   dataDir,
		CacheDir:  cacheDir,
		PrefixDir: filepath.Join(dataDir, "prefix"),
		GamesDir:  filepath.Join(home, "Games", "battlenet"),
		LogFile:   filepath.Join(cacheDir, "bnetctl.log"),
	}, nil
}
