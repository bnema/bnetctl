package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/env"
)

// registerDisplayDriverFlag adds the shared Wine display driver option.
func registerDisplayDriverFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "display-driver", string(env.DisplayDriverWayland), "Wine display driver: wayland or x11")
}

// resolveDisplayDriver validates the requested driver and confirms the matching
// display server is reachable before any Wine process starts.
func resolveDisplayDriver(raw string) (env.DisplayDriver, error) {
	driver := env.DisplayDriver(raw)
	if driver != env.DisplayDriverWayland && driver != env.DisplayDriverX11 {
		return "", fmt.Errorf("invalid display driver %q: use wayland or x11", raw)
	}
	if driver == env.DisplayDriverWayland && !env.IsWayland() {
		return "", fmt.Errorf("wayland display unavailable: WAYLAND_DISPLAY is not set")
	}
	if driver == env.DisplayDriverX11 && os.Getenv("DISPLAY") == "" {
		return "", fmt.Errorf("x11 display unavailable: DISPLAY is not set")
	}
	return driver, nil
}
