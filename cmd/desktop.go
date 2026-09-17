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
	Short: "Manage the .desktop entries for Battle.net",
}

var desktopAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create .desktop entries for Battle.net (Wayland and X11)",
	RunE:  runDesktopAdd,
}

var desktopRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove the .desktop entries for Battle.net",
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

	if desktop.EntryExists(ports.EntryWayland) && desktop.EntryExists(ports.EntryX11) {
		fmt.Println(styles.Warning.Render("Desktop entries already exist."))
		return nil
	}

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	extractor := icoutils.NewExtractor(log)
	exePath := filepath.Join(cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	destPath := filepath.Join(cfg.DataDir, "battlenet.png")
	iconPath := extractor.ExtractIcon(exePath, destPath)
	if err := createDesktopEntries(desktop, iconPath); err != nil {
		return err
	}

	log.Info("desktop entries created", "icon", iconPath)
	fmt.Println(styles.Success.Render("Desktop entries created!"))
	fmt.Println(styles.Muted.Render("  Battle.net (Wayland and X11) should now appear in your application menu."))
	return nil
}

func runDesktopRemove(cmd *cobra.Command, args []string) error {
	log := getLogger()

	desktop := xdg.NewDesktop()

	if !desktop.EntryExists(ports.EntryWayland) &&
		!desktop.EntryExists(ports.EntryX11) &&
		!desktop.EntryExists(ports.EntryLegacy) {
		fmt.Println(styles.Muted.Render("No desktop entry found."))
		return nil
	}

	if err := removeDesktopEntries(desktop); err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Failed to remove desktop entry: ")+err.Error())
		return err
	}

	log.Info("desktop entries removed")
	fmt.Println(styles.Success.Render("Desktop entries removed."))
	return nil
}

// createDesktopEntries creates the Wayland and X11 .desktop files sharing the
// given icon path. The executable path is quoted per Desktop Entry spec.
func createDesktopEntries(desktop ports.DesktopPort, iconPath string) error {
	if err := desktop.RemoveEntry(ports.EntryLegacy); err != nil {
		return fmt.Errorf("remove legacy desktop entry: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "bnetctl"
	}

	waylandCmd := xdg.QuoteDesktopExec(execPath, "launch", "--display-driver", "wayland")
	x11Cmd := xdg.QuoteDesktopExec(execPath, "launch", "--display-driver", "x11")

	if err := desktop.CreateEntry(ports.EntryWayland, ports.DisplayNameWayland, waylandCmd, iconPath); err != nil {
		return fmt.Errorf("create wayland desktop entry: %w", err)
	}
	if err := desktop.CreateEntry(ports.EntryX11, ports.DisplayNameX11, x11Cmd, iconPath); err != nil {
		return fmt.Errorf("create x11 desktop entry: %w", err)
	}
	return nil
}

// removeDesktopEntries removes both current entries plus the legacy
// single entry from earlier versions.
func removeDesktopEntries(desktop ports.DesktopPort) error {
	for _, name := range []string{ports.EntryWayland, ports.EntryX11, ports.EntryLegacy} {
		if err := desktop.RemoveEntry(name); err != nil {
			return fmt.Errorf("remove desktop entry %s: %w", name, err)
		}
	}
	return nil
}
