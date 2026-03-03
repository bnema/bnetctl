package logger

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

// Log is the global logger instance
var Log *log.Logger

var logFile *os.File

// Init initializes the logger. If verbose is true, logs go to stderr too.
func Init(verbose bool) error {
	// Default: discard logs
	Log = log.New(os.Stderr)
	Log.SetLevel(log.InfoLevel)

	if !verbose {
		Log.SetLevel(log.WarnLevel)
	} else {
		Log.SetLevel(log.DebugLevel)
	}

	return nil
}

// InitWithFile initializes logging to a file
func InitWithFile(logPath string, verbose bool) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	Log = log.New(logFile)
	Log.SetLevel(log.DebugLevel)

	if verbose {
		// Also log to stderr when verbose
		stderrLogger := log.New(os.Stderr)
		stderrLogger.SetLevel(log.DebugLevel)
		Log = stderrLogger
	}

	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}
