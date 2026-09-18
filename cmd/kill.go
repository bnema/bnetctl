package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
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
	result, err := runtime.GracefulKillPrefix(cfg.PrefixDir, killTimeout)
	printStopResult(result)
	if err != nil {
		log.Error("kill failed", "error", err)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Kill incomplete: ")+err.Error())
		return err
	}

	log.Info("wine processes stopped")
	fmt.Println(styles.Success.Render("Done."))
	return nil
}

// printStopResult reports what the stop terminated, so a kill that found nothing
// running is not mistaken for a shutdown that did something.
func printStopResult(result domain.StopResult) {
	if result.WineserverStopped {
		fmt.Println("  " + styles.Muted.Render("stopped the wineserver"))
	}
	for _, process := range result.Leftovers {
		fmt.Println("  " + styles.Muted.Render("killed leftover "+process.String()))
	}
	if !result.WineserverStopped && len(result.Leftovers) == 0 {
		fmt.Println("  " + styles.Muted.Render("nothing was running"))
	}
}
