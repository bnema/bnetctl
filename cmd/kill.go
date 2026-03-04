package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var killAll bool

var killCmd = &cobra.Command{
	Use:     "kill",
	Aliases: []string{"stop"},
	Short:   "Kill all Battle.net/Wine processes",
	Long: `Stop all running Battle.net and Wine processes for the bnetctl prefix.

Use --all to also kill orphaned Wine system processes (explorer.exe, services.exe, etc.)
that may have survived previous runs.`,
	RunE: runKill,
}

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill everything including orphaned system processes")
	rootCmd.AddCommand(killCmd)
}

func runKill(cmd *cobra.Command, args []string) error {
	log := getLogger()

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		log.Error("load config failed", "error", err)
		return fmt.Errorf("load config: %w", err)
	}

	runtime := wine.NewAdapter()
	log.Info("killing wine processes", "prefix", cfg.PrefixDir, "all", killAll)

	// Step 1: Try graceful wineserver kill
	fmt.Println("Stopping wineserver...")
	_ = runtime.GracefulKillPrefix(cfg.PrefixDir, 3*time.Second)
	log.Info("wineserver stopped")

	if !killAll {
		fmt.Println(styles.Success.Render("Done."))
		return nil
	}

	// Step 2: --all mode — kill any orphaned processes belonging to our prefix
	fmt.Println("Scanning for orphaned processes...")
	killed, err := runtime.KillOrphans(cfg.PrefixDir)
	if err != nil {
		log.Warn("orphan scan failed", "error", err)
	}
	log.Info("orphan scan complete", "killed", len(killed))
	if len(killed) > 0 {
		for _, pid := range killed {
			fmt.Printf("  killed PID %d\n", pid)
		}
		fmt.Printf(styles.Success.Render("Killed %d orphaned process(es).")+"\n", len(killed))
	} else {
		fmt.Println(styles.Success.Render("No orphaned processes found."))
	}

	return nil
}
