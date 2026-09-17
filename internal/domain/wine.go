package domain

import (
	"os"
	"os/user"
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
