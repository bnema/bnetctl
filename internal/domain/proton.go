package domain

// ProtonRuntime represents a detected Proton installation
type ProtonRuntime struct {
	// BasePath is the root directory of the Proton installation
	BasePath string
	// BinDir is the directory containing wine/wine64/wineserver binaries
	BinDir string
	// LibDir is the directory containing libraries
	LibDir string
	// DistDir is the files/ subdirectory
	DistDir string
	// DefaultPrefixDir is the default prefix template
	DefaultPrefixDir string
	// Version is the Proton version string
	Version string
}

// ProtonEnv holds the environment variables needed to run Proton standalone
type ProtonEnv struct {
	// Vars is the map of environment variables to set
	Vars map[string]string
}

// DefaultProtonPath is where proton-cachyos is installed on CachyOS/Arch
const DefaultProtonPath = "/usr/share/steam/compatibilitytools.d/proton-cachyos"

// ProtonVerbs are the supported Proton command verbs
const (
	VerbRun               = "run"
	VerbWaitForExitAndRun = "waitforexitandrun"
	VerbRunInPrefix       = "runinprefix"
)
