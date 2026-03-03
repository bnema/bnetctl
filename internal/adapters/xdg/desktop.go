package xdg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bnema/bnetctl/internal/ports"
)

// Desktop implements ports.DesktopPort using XDG desktop entries
type Desktop struct{}

var _ ports.DesktopPort = (*Desktop)(nil)

// NewDesktop creates a new XDG desktop adapter
func NewDesktop() *Desktop {
	return &Desktop{}
}

// CreateEntry creates a .desktop file for the application
func (d *Desktop) CreateEntry(name string, execCmd string, iconPath string) error {
	appsDir, err := desktopAppsDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return fmt.Errorf("create applications dir: %w", err)
	}

	desktopFile := filepath.Join(appsDir, name+".desktop")

	content := fmt.Sprintf(`[Desktop Entry]
Name=Battle.net
Comment=Battle.net Game Launcher (via bnetctl)
Exec=%s
Icon=%s
Terminal=false
Type=Application
Categories=Game;
StartupWMClass=battle.net.exe
`, execCmd, iconPath)

	if err := os.WriteFile(desktopFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write desktop file: %w", err)
	}

	return nil
}

// RemoveEntry removes the .desktop file
func (d *Desktop) RemoveEntry(name string) error {
	appsDir, err := desktopAppsDir()
	if err != nil {
		return err
	}

	desktopFile := filepath.Join(appsDir, name+".desktop")
	if err := os.Remove(desktopFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove desktop file: %w", err)
	}
	return nil
}

// EntryExists checks if the .desktop file exists
func (d *Desktop) EntryExists(name string) bool {
	appsDir, err := desktopAppsDir()
	if err != nil {
		return false
	}

	desktopFile := filepath.Join(appsDir, name+".desktop")
	_, err = os.Stat(desktopFile)
	return err == nil
}

func desktopAppsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "applications"), nil
}
