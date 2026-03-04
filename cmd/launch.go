package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/env"
	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/services"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var launchCmd = &cobra.Command{
	Use:     "launch",
	Aliases: []string{"start", "run", "play"},
	Short:   "Start Battle.net",
	RunE:    runLaunch,
}

func init() {
	rootCmd.AddCommand(launchCmd)
}

func runLaunch(cmd *cobra.Command, args []string) error {
	log := getLogger()

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	runtime := wine.NewAdapter()
	fs := xdg.NewFilesystem()

	fmt.Println(styles.StepPrefix.Render("Launching Battle.net..."))

	launcher := services.NewLaunchService(runtime, fs, cfg, env.BuildGPUEnv)

	// Launch replaces the process on success
	log.Info("launching battle.net")
	result, err := launcher.Launch()
	if err != nil {
		log.Error("launch failed", "error", err)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Launch failed: ")+err.Error())
		return err
	}
	if result != nil && result.Runtime != nil {
		fmt.Println(styles.StepPrefix.Render("Wine: ") + result.Runtime.Version)
	}

	log.Info("battle.net exited")
	// Should not reach here (process replaced)
	return nil
}
