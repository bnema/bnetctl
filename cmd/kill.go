package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

// killTimeout is how long the wineserver gets to stop on its own before it is
// force killed. Battle.net needs that window to persist its session.
const killTimeout = 10 * time.Second

var killCmd = &cobra.Command{
	Use:          "kill",
	Aliases:      []string{"stop"},
	Short:        "Kill all Battle.net/Wine processes",
	SilenceUsage: true,
	Long: `Stop all Battle.net and Wine processes for the bnetctl prefix.

Processes left behind without a wineserver (explorer.exe, services.exe,
Battle.net Helper.exe, ...) are reaped as well.`,
	RunE: runKill,
}

func init() {
	rootCmd.AddCommand(killCmd)
}

func runKill(cmd *cobra.Command, args []string) error {
	log := getLogger()

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		log.Error("load config failed", "error", err)
		return fmt.Errorf("load config: %w", err)
	}

	runtime := wine.NewAdapter(log)
	log.Info("killing wine processes", "prefix", cfg.PrefixDir)

	fmt.Println("Stopping Battle.net and Wine processes...")
	if err := runtime.GracefulKillPrefix(cfg.PrefixDir, killTimeout); err != nil {
		log.Error("kill failed", "error", err)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Kill incomplete: ")+err.Error())
		return err
	}

	log.Info("wine processes stopped")
	fmt.Println(styles.Success.Render("Done."))
	return nil
}
