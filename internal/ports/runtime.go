package ports

import (
	"time"

	"github.com/bnema/bnetctl/internal/domain"
)

// RuntimePort defines operations for running Windows executables via a compatibility layer
type RuntimePort interface {
	// Detect checks if wine-cachyos is available on the system
	Detect() (*domain.WineRuntime, error)

	// CreatePrefix initializes a new Wine prefix at the given path
	CreatePrefix(prefixPath string) error

	// DisableSystray disables Wine's standalone systray window for the prefix.
	// It must be applied before an application registers a tray icon.
	DisableSystray(prefixPath string) error

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
