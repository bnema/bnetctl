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
	printStopResult(result, err == nil)
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
// running is not mistaken for a shutdown that did something. verified is false when
// the stop could not be confirmed, in which case nothing positive is claimed.
func printStopResult(result domain.StopResult, verified bool) {
	if result.Wineserver != nil {
		fmt.Println("  " + styles.Muted.Render("stopped "+result.Wineserver.String()))
	}
	if len(result.Session) > 0 {
		line := pluralProcesses(len(result.Session)) + ": " + result.Session.String()
		fmt.Println("  " + styles.Muted.Render("stopped "+line))
	}
	for _, process := range result.Leftovers {
		fmt.Println("  " + styles.Muted.Render("killed "+process.String()))
	}

	idle := result.Wineserver == nil && len(result.Session) == 0 && len(result.Leftovers) == 0
	if verified && idle {
		fmt.Println("  " + styles.Muted.Render("nothing was running"))
	}
}

// pluralProcesses formats a process count, for example "1 wine process".
func pluralProcesses(count int) string {
	if count == 1 {
		return "1 wine process"
	}
	return fmt.Sprintf("%d wine processes", count)
}
