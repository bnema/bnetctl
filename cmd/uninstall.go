package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the desktop entry",
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	desktop := xdg.NewDesktop()

	if !desktop.EntryExists("bnetctl") {
		fmt.Println(styles.Muted.Render("No desktop entry found."))
		return nil
	}

	if err := desktop.RemoveEntry("bnetctl"); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to remove desktop entry: ")+err.Error())
		return err
	}

	fmt.Println(styles.Success.Render("Desktop entry removed."))
	return nil
}
