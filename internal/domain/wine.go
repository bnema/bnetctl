package domain

import (
	"fmt"
	"os"
	"os/user"
	"strings"
)

// WineRuntime represents a detected Wine installation
type WineRuntime struct {
	// WineBin is the path to the wine binary
	WineBin string
	// WineBootBin is the path to the wineboot binary
	WineBootBin string
	// WineServerBin is the path to the wineserver binary
	WineServerBin string
	// Version is the Wine version string
	Version string
	// HasNTSync indicates if /dev/ntsync is available
	HasNTSync bool
	// HasDXVKSetup indicates if setup_dxvk is on PATH
	HasDXVKSetup bool
	// DXVKSetupBin is the path to setup_dxvk (empty if not found)
	DXVKSetupBin string
	// HasVKD3DSetup indicates if setup_vkd3d_proton is on PATH
	HasVKD3DSetup bool
	// VKD3DSetupBin is the path to setup_vkd3d_proton (empty if not found)
	VKD3DSetupBin string
}

// WineEnv holds the environment variables needed to run Wine
type WineEnv struct {
	// Vars is the map of environment variables to set.
	Vars map[string]string
	// Unset lists inherited environment variables to remove.
	Unset []string
}

// StoppedProcess describes a wine process involved in stopping a prefix.
type StoppedProcess struct {
	// PID is the process id
	PID int
	// Name is the process name as the kernel reports it
	Name string
}

// String formats the process for display, for example "explorer.exe (pid 1234)".
func (p StoppedProcess) String() string {
	return fmt.Sprintf("%s (pid %d)", p.Name, p.PID)
}

// StoppedProcesses is a list of processes, formatted for display.
type StoppedProcesses []StoppedProcess

// String joins the processes, for example "wineserver (pid 9), explorer.exe (pid 42)".
func (p StoppedProcesses) String() string {
	described := make([]string, 0, len(p))
	for _, process := range p {
		described = append(described, process.String())
	}
	return strings.Join(described, ", ")
}

// StopResult reports what stopping a prefix terminated.
type StopResult struct {
	// Wineserver is the wineserver that was stopped, nil when none was running
	Wineserver *StoppedProcess
	// Session lists the wine processes the wineserver was holding when the stop
	// started. The wineserver terminates them itself, so they cannot be observed
	// afterwards.
	Session StoppedProcesses
	// Leftovers lists the processes without a wineserver that the sweep killed
	Leftovers StoppedProcesses
}

// WineUsername returns the current Linux username (used for Wine prefix user paths).
// The Wine prefix user directory matches the Linux username.
func WineUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "unknown"
}
