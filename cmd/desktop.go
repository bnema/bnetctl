package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/icoutils"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Manage the .desktop entry for Battle.net",
}

var desktopAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a .desktop entry for Battle.net",
	RunE:  runDesktopAdd,
}

var desktopRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove the .desktop entry for Battle.net",
	RunE:    runDesktopRemove,
}

func init() {
	desktopCmd.AddCommand(desktopAddCmd)
	desktopCmd.AddCommand(desktopRemoveCmd)
	rootCmd.AddCommand(desktopCmd)
}

func runDesktopAdd(cmd *cobra.Command, args []string) error {
	log := getLogger()

	desktop := xdg.NewDesktop()

	if desktop.EntryExists("bnetctl") {
		fmt.Println(styles.Warning.Render("Desktop entry already exists."))
		return nil
	}

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	extractor := icoutils.NewExtractor(getLogger())
	exePath := filepath.Join(cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	destPath := filepath.Join(cfg.DataDir, "battlenet.png")
	iconPath := extractor.ExtractIcon(exePath, destPath)
	if err := createDesktopEntry(desktop, iconPath); err != nil {
		return err
	}

	log.Info("desktop entry created", "icon", iconPath)
	fmt.Println(styles.Success.Render("Desktop entry created!"))
	fmt.Println(styles.Muted.Render("  Battle.net should now appear in your application menu."))
	return nil
}

func runDesktopRemove(cmd *cobra.Command, args []string) error {
	log := getLogger()

	desktop := xdg.NewDesktop()

	if !desktop.EntryExists("bnetctl") {
		fmt.Println(styles.Muted.Render("No desktop entry found."))
		return nil
	}

	if err := desktop.RemoveEntry("bnetctl"); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to remove desktop entry: ")+err.Error())
		return err
	}

	log.Info("desktop entry removed")
	fmt.Println(styles.Success.Render("Desktop entry removed."))
	return nil
}

// createDesktopEntry creates the .desktop file with the given icon path.
func createDesktopEntry(desktop ports.DesktopPort, iconPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "bnetctl"
	}
	execCmd := execPath + " launch"

	if err := desktop.CreateEntry("bnetctl", execCmd, iconPath); err != nil {
		return fmt.Errorf("create desktop entry: %w", err)
	}
	return nil
}
