package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/env"
	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/services"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var displayDriver string

var launchCmd = &cobra.Command{
	Use:     "launch",
	Aliases: []string{"start", "run", "play"},
	Short:   "Start Battle.net",
	RunE:    runLaunch,
}

func init() {
	launchCmd.Flags().StringVar(&displayDriver, "display-driver", string(env.DisplayDriverWayland), "Wine display driver: wayland or x11")
	rootCmd.AddCommand(launchCmd)
}

func runLaunch(cmd *cobra.Command, args []string) error {
	log := getLogger()

	driver := env.DisplayDriver(displayDriver)
	if driver != env.DisplayDriverWayland && driver != env.DisplayDriverX11 {
		return fmt.Errorf("invalid display driver %q: use wayland or x11", displayDriver)
	}
	if driver == env.DisplayDriverWayland && !env.IsWayland() {
		return fmt.Errorf("wayland display unavailable: WAYLAND_DISPLAY is not set")
	}
	if driver == env.DisplayDriverX11 && os.Getenv("DISPLAY") == "" {
		return fmt.Errorf("x11 display unavailable: DISPLAY is not set")
	}

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	runtime := wine.NewAdapter(log)
	fs := xdg.NewFilesystem()

	// Display Wine version before launch (display-only; service re-detects internally)
	if rt, err := runtime.Detect(); err == nil {
		fmt.Println(styles.StepPrefix.Render("Wine: ") + rt.Version)
	}
	fmt.Println(styles.StepPrefix.Render("Launching Battle.net..."))

	launcher := services.NewLaunchService(runtime, fs, cfg, func() *domain.WineEnv {
		return env.BuildGPUEnvForDisplay(driver)
	}, log, env.BattleNetArgsForDisplay(driver)...)

	log.Info("launching battle.net")
	if _, err := launcher.Launch(); err != nil {
		log.Error("launch failed", "error", err)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Launch failed: ")+err.Error())
		return err
	}

	log.Info("battle.net exited")
	// Should not reach here (process replaced)
	return nil
}
