package ports

// IconExtractorPort extracts application icons from Windows executables.
type IconExtractorPort interface {
	// ExtractIcon extracts the best-quality icon from exePath and writes it to destPath.
	// Returns the final icon path on success, or a fallback XDG icon name on failure.
	ExtractIcon(exePath string, destPath string) (string, error)
}
