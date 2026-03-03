package ports

// DesktopPort defines operations for desktop integration (menu entries, icons)
type DesktopPort interface {
	// CreateEntry creates a .desktop file for the application
	CreateEntry(name string, execCmd string, iconPath string) error

	// RemoveEntry removes the .desktop file
	RemoveEntry(name string) error

	// EntryExists checks if the .desktop file already exists
	EntryExists(name string) bool
}
