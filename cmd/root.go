package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/logger"
)

// Version info set via ldflags at build time
var (
	version = "dev"
	commit  = "unknown"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:     "bnetctl",
	Short:   "Battle.net launcher for Linux via wine-cachyos",
	Version: version + " (" + commit + ")",
	Long: `A CLI tool to install, manage, and run Battle.net on Linux
using wine-cachyos as the compatibility layer.

Quick start:
  bnetctl install    Download and install Battle.net
  bnetctl launch     Start Battle.net`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	logger.Close()
}

func init() {
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		logFile, err := xdg.LogFilePath()
		if err != nil {
			// Fall back to stderr-only logging
			_ = logger.Init(verbose)
			return
		}
		if err := logger.InitWithFile(logFile, verbose); err != nil {
			// Fall back to stderr-only logging
			_ = logger.Init(verbose)
		}
	}
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
}

// getLogger returns the global logger
func getLogger() *log.Logger {
	return logger.Log
}
