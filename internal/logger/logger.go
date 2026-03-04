package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

// Log is the global logger instance
var Log *log.Logger

var logFile *os.File

// Init initializes the logger with file output and optional stderr output.
// Logs always go to logPath. When verbose is true, logs also go to stderr.
func Init(verbose bool) error {
	// Fallback: stderr-only logger until InitWithFile is called
	Log = log.New(os.Stderr)
	if verbose {
		Log.SetLevel(log.DebugLevel)
	} else {
		Log.SetLevel(log.WarnLevel)
	}
	return nil
}

// InitWithFile initializes logging to a file (always) and stderr (when verbose).
func InitWithFile(logPath string, verbose bool) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	// Always write to file; also to stderr when verbose
	var w io.Writer
	if verbose {
		w = io.MultiWriter(logFile, os.Stderr)
	} else {
		w = logFile
	}

	Log = log.New(w)
	Log.SetLevel(log.DebugLevel)

	return nil
}

// Close closes the log file
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}
