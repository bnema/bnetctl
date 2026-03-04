package ports

import (
	"os"

	"github.com/bnema/bnetctl/internal/domain"
)

// FilesystemPort defines operations for filesystem and directory management
type FilesystemPort interface {
	// EnsureDirs creates all required directories if they don't exist
	EnsureDirs(cfg *domain.Config) error

	// Exists checks if a path exists
	Exists(path string) bool

	// Remove removes a file or directory recursively
	Remove(path string) error

	// HomeDir returns the user's home directory
	HomeDir() (string, error)

	// MkdirAll creates a directory and all parent directories
	MkdirAll(path string, perm os.FileMode) error

	// ReadFile reads the full contents of a file
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to a file with the given permissions
	WriteFile(path string, data []byte, perm os.FileMode) error
}
