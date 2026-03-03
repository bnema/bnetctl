package domain

// Installation represents the state of a Battle.net installation
type Installation struct {
	// PrefixPath is the Wine/Proton prefix directory
	PrefixPath string
	// ExePath is the path to Battle.net.exe inside the prefix
	ExePath string
	// SetupPath is the path to the downloaded Battle.net-Setup.exe
	SetupPath string
	// Installed indicates whether Battle.net has been installed
	Installed bool
}

// Config holds application configuration
type Config struct {
	// DataDir is the base data directory (~/.local/share/bnetctl)
	DataDir string
	// CacheDir is the cache directory (~/.cache/bnetctl)
	CacheDir string
	// PrefixDir is the Wine prefix directory (~/.local/share/bnetctl/prefix)
	PrefixDir string
	// GamesDir is where games are installed (~/Games/battlenet)
	GamesDir string
	// LogFile is the path to the log file
	LogFile string
}

// BattleNetSetupURL is the official download URL for Battle.net installer
const BattleNetSetupURL = "https://downloader.battle.net/download/getInstaller?os=win&installer=Battle.net-Setup.exe"

// BattleNetExeRelPath is the relative path to Battle.net.exe inside the prefix
const BattleNetExeRelPath = "drive_c/Program Files (x86)/Battle.net/Battle.net.exe"
