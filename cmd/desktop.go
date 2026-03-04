package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/logger"
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
	log := logger.Log

	desktop := xdg.NewDesktop()

	if desktop.EntryExists("bnetctl") {
		fmt.Println(styles.Warning.Render("Desktop entry already exists."))
		return nil
	}

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	iconPath := extractIcon(cfg)
	if err := createDesktopEntry(desktop, iconPath); err != nil {
		return err
	}

	log.Info("desktop entry created", "icon", iconPath)
	fmt.Println(styles.Success.Render("Desktop entry created!"))
	fmt.Println(styles.Muted.Render("  Battle.net should now appear in your application menu."))
	return nil
}

func runDesktopRemove(cmd *cobra.Command, args []string) error {
	log := logger.Log

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
func createDesktopEntry(desktop *xdg.Desktop, iconPath string) error {
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

// extractIcon attempts to extract the Battle.net icon from Battle.net.exe
// using wrestool + icotool (from icoutils package). Returns the icon path
// on success, or a generic fallback icon name if extraction fails.
func extractIcon(cfg *domain.Config) string {
	log := logger.Log

	destPath := filepath.Join(cfg.DataDir, "battlenet.png")

	// If icon already exists, reuse it
	if _, err := os.Stat(destPath); err == nil {
		return destPath
	}

	exePath := filepath.Join(cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
	if _, err := os.Stat(exePath); err != nil {
		log.Debug("battle.net exe not found, using fallback icon")
		return "applications-games"
	}

	// Check if icoutils is available
	wrestool, err := exec.LookPath("wrestool")
	if err != nil {
		log.Debug("wrestool not found, using fallback icon", "hint", "install icoutils for Battle.net icon extraction")
		return "applications-games"
	}
	icotool, err := exec.LookPath("icotool")
	if err != nil {
		log.Debug("icotool not found, using fallback icon", "hint", "install icoutils for Battle.net icon extraction")
		return "applications-games"
	}

	// Extract .ico from exe
	tmpIco := filepath.Join(os.TempDir(), "bnetctl-icon.ico")
	defer os.Remove(tmpIco)

	// wrestool -x -t 14 -o /tmp/bnetctl-icon.ico "Battle.net.exe"
	// Type 14 = RT_GROUP_ICON
	wrestoolCmd := exec.Command(wrestool, "-x", "-t", "14", "-o", tmpIco, exePath)
	if err := wrestoolCmd.Run(); err != nil {
		log.Debug("wrestool extraction failed", "error", err)
		return "applications-games"
	}

	// Convert .ico to .png (pick largest resolution)
	// icotool -x -o /tmp/ /tmp/bnetctl-icon.ico produces multiple PNGs
	tmpDir, err := os.MkdirTemp("", "bnetctl-icons-")
	if err != nil {
		return "applications-games"
	}
	defer os.RemoveAll(tmpDir)

	icotoolCmd := exec.Command(icotool, "-x", "-o", tmpDir, tmpIco)
	if err := icotoolCmd.Run(); err != nil {
		log.Debug("icotool conversion failed", "error", err)
		return "applications-games"
	}

	// Find the largest PNG extracted
	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return "applications-games"
	}

	var bestFile string
	var bestSize int64
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".png" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			bestFile = filepath.Join(tmpDir, entry.Name())
		}
	}

	if bestFile == "" {
		return "applications-games"
	}

	// Copy best icon to destination
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "applications-games"
	}

	data, err := os.ReadFile(bestFile)
	if err != nil {
		return "applications-games"
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return "applications-games"
	}

	log.Info("extracted battle.net icon", "path", destPath, "size", bestSize)
	return destPath
}
