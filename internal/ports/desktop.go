package ports

// Desktop entry identifiers (desktop filenames without the .desktop suffix)
// and their display names.
const (
	// EntryWayland is the desktop file base for the Wayland launcher.
	EntryWayland = "bnetctl-wayland"
	// EntryX11 is the desktop file base for the X11 launcher.
	EntryX11 = "bnetctl-x11"
	// EntryLegacy is the historical single-entry desktop file base.
	EntryLegacy = "bnetctl"

	// DisplayNameWayland is shown exactly in menus for the Wayland entry.
	DisplayNameWayland = "Battle.net (bnetctl/Wayland)"
	// DisplayNameX11 is shown exactly in menus for the X11 entry.
	DisplayNameX11 = "Battle.net (bnetctl/X11)"
)

// DesktopPort defines operations for desktop integration (menu entries, icons)
type DesktopPort interface {
	// CreateEntry creates a .desktop file for the application.
	// name is the desktop file base (without .desktop),
	// displayName is the exact Name= value shown in menus.
	CreateEntry(name string, displayName string, execCmd string, iconPath string) error

	// RemoveEntry removes the .desktop file
	RemoveEntry(name string) error

	// EntryExists checks if the .desktop file already exists
	EntryExists(name string) bool
}
