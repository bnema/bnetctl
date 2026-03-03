package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/proton"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/services"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var cleanAll bool

var cleanCmd = &cobra.Command{
	Use:     "clean",
	Aliases: []string{"c"},
	Short:   "Remove bnetctl data (cache, prefix)",
	Long: `Remove cached files and Wine prefix.
Use -a/--all to remove everything including desktop entry and data directory.`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVarP(&cleanAll, "all", "a", false, "Remove everything (cache, prefix, desktop entry, data)")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	runtime := proton.NewAdapter("")
	desktop := xdg.NewDesktop()
	fs := xdg.NewFilesystem()

	cleaner := services.NewCleanerService(runtime, desktop, fs, cfg)

	if cleanAll {
		fmt.Println(styles.Warning.Render("Removing all bnetctl data..."))
		if err := cleaner.CleanAll(); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Clean failed: ")+err.Error())
			return err
		}
		fmt.Println(styles.Success.Render("All bnetctl data removed."))
	} else {
		fmt.Println(styles.Warning.Render("Removing cache and prefix..."))
		if err := cleaner.CleanCache(); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Cache clean failed: ")+err.Error())
		}
		if err := cleaner.CleanPrefix(); err != nil {
			fmt.Fprintln(os.Stderr, styles.Error.Render("Prefix clean failed: ")+err.Error())
			return err
		}
		fmt.Println(styles.Success.Render("Cache and prefix removed."))
	}

	return nil
}
