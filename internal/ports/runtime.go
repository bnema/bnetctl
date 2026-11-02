package ports

import (
	"time"

	"github.com/bnema/bnetctl/internal/domain"
)

// RuntimePort defines operations for running Windows executables via a compatibility layer
type RuntimePort interface {
	// WithStoppedPrefix verifies shutdown and holds exclusion through remove.
	// It does not perform migration preflight; remove is skipped on stop failure.
	WithStoppedPrefix(prefixPath string, timeout time.Duration, remove func() error) error

	// LockPrefix locks maintenance through process start and selects a non-downgrading runtime.
	LockPrefix(prefixPath string) (release func(), err error)

	// Detect checks if wine-cachyos is available on the system
	Detect() (*domain.WineRuntime, error)

	// UpdatePrefix updates the prefix and graphics when the selected Wine version changes.
	UpdatePrefix(prefixPath string, env *domain.WineEnv) error

	// CreatePrefix initializes a new Wine prefix at the given path
	CreatePrefix(prefixPath string) error

	// DisableSystray disables Wine's standalone systray window for the prefix.
	// When provided, env must match the following application's environment.
	DisableSystray(prefixPath string, env *domain.WineEnv) error

	// RunExe runs a Windows executable inside the prefix (blocking)
	// exePath is the path to the .exe file
	RunExe(prefixPath string, exePath string, env *domain.WineEnv) error

	// RunExeAsync runs a Windows executable inside the prefix without blocking.
	// Returns a channel that receives the exit error when the process completes.
	RunExeAsync(prefixPath string, exePath string, env *domain.WineEnv, args ...string) (<-chan error, error)

	// IsProcessRunning checks if a Wine process is running in the given prefix
	IsProcessRunning(prefixPath string) bool

	// GracefulKillPrefix stops everything running in the prefix, waits up to
	// timeout for the wineserver to exit before force killing it, and sweeps
	// processes left without a wineserver. The returned result reports what was
	// stopped, and an error reports what survived.
	GracefulKillPrefix(prefixPath string, timeout time.Duration) (domain.StopResult, error)
}
