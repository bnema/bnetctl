package ports

import (
	"time"

	"github.com/bnema/bnetctl/internal/domain"
)

// RuntimePort defines operations for running Windows executables via a compatibility layer
type RuntimePort interface {
	// Detect checks if the runtime (e.g., proton-cachyos) is available on the system
	Detect() (*domain.ProtonRuntime, error)

	// CreatePrefix initializes a new Wine/Proton prefix at the given path
	CreatePrefix(prefixPath string) error

	// RunExe runs a Windows executable inside the prefix (blocking)
	// verb is the Proton verb (run, waitforexitandrun, runinprefix)
	// exePath is the path to the .exe file
	RunExe(verb string, prefixPath string, exePath string, env *domain.ProtonEnv) error

	// RunExeAsync runs a Windows executable inside the prefix without blocking.
	// Returns a channel that receives the exit error when the process completes.
	RunExeAsync(verb string, prefixPath string, exePath string, env *domain.ProtonEnv) (<-chan error, error)

	// IsProcessRunning checks if a Wine process is running in the given prefix
	IsProcessRunning(prefixPath string) bool

	// KillPrefix stops all Wine processes in the given prefix
	KillPrefix(prefixPath string) error

	// WaitPrefix blocks until the wineserver for the given prefix exits
	WaitPrefix(prefixPath string) error

	// GracefulKillPrefix attempts a graceful stop (SIGTERM), waits up to timeout,
	// then force kills (SIGKILL) any remaining processes in the prefix.
	GracefulKillPrefix(prefixPath string, timeout time.Duration) error
}
