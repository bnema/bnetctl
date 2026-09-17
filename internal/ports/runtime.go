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

	// KillPrefix stops all Wine processes in the given prefix
	KillPrefix(prefixPath string) error

	// WaitPrefix blocks until the wineserver for the given prefix exits
	WaitPrefix(prefixPath string) error

	// GracefulKillPrefix attempts a graceful stop (SIGTERM), waits up to timeout,
	// then force kills (SIGKILL) any remaining processes in the prefix.
	GracefulKillPrefix(prefixPath string, timeout time.Duration) error

	// KillOrphans scans for and kills any processes belonging to the given prefix
	// that survived wineserver shutdown. Returns the PIDs of killed processes.
	KillOrphans(prefixPath string) ([]int, error)
}
