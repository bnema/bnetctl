package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Create a .desktop entry for Battle.net",
	RunE:  runDesktop,
}

func init() {
	rootCmd.AddCommand(desktopCmd)
}

func runDesktop(cmd *cobra.Command, args []string) error {
	desktop := xdg.NewDesktop()

	if desktop.EntryExists("bnetctl") {
		fmt.Println(styles.Warning.Render("Desktop entry already exists."))
		return nil
	}

	// Find bnetctl executable path
	execPath, err := os.Executable()
	if err != nil {
		execPath = "bnetctl"
	}
	execCmd := execPath + " launch"

	// Use a default icon path (user can customize later)
	home, _ := os.UserHomeDir()
	iconPath := filepath.Join(home, ".local", "share", "bnetctl", "battlenet.png")

	// If icon doesn't exist, use a generic name
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = "applications-games"
	}

	if err := desktop.CreateEntry("bnetctl", execCmd, iconPath); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to create desktop entry: ")+err.Error())
		return err
	}

	fmt.Println(styles.Success.Render("Desktop entry created!"))
	fmt.Println(styles.Muted.Render("  Battle.net should now appear in your application menu."))
	return nil
}
