package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/http"
	"github.com/bnema/bnetctl/internal/adapters/proton"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/services"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var installCmd = &cobra.Command{
	Use:     "install",
	Aliases: []string{"i"},
	Short:   "Download and install Battle.net",
	RunE:    runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	log := getLogger()

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	runtime := proton.NewAdapter("")
	downloader := http.NewDownloader()
	fs := xdg.NewFilesystem()

	// Detect proton first
	rt, err := runtime.Detect()
	if err != nil {
		fmt.Fprintln(os.Stderr, styles.Error.Render("Error: ")+err.Error())
		fmt.Fprintln(os.Stderr, styles.Muted.Render("Install proton-cachyos: paru -S proton-cachyos"))
		return err
	}
	fmt.Println(styles.StepPrefix.Render("Proton: ") + rt.Version)

	installer := services.NewInstallerService(runtime, downloader, fs, cfg)

	// Check if already installed
	inst := installer.GetInstallation()
	if inst.Installed {
		fmt.Println(styles.Warning.Render("Battle.net is already installed."))
		return nil
	}

	// Track last status to avoid repeating messages
	var lastStatus services.InstallStatus = -1

	progressFn := func(p services.InstallProgress) {
		switch p.Status {
		case services.InstallDownloading:
			if p.Download != nil {
				if p.Download.TotalBytes > 0 {
					mb := float64(p.Download.BytesDownloaded) / 1024 / 1024
					totalMb := float64(p.Download.TotalBytes) / 1024 / 1024
					fmt.Fprintf(os.Stdout, "\r  Downloading: %.1f / %.1f MB (%.0f%%)", mb, totalMb, p.Download.Percent)
				} else {
					mb := float64(p.Download.BytesDownloaded) / 1024 / 1024
					fmt.Fprintf(os.Stdout, "\r  Downloading: %.1f MB", mb)
				}
			}
		case services.InstallCreatingPrefix:
			if lastStatus != p.Status {
				fmt.Println()
				fmt.Println(styles.StepPrefix.Render("  Creating Wine prefix..."))
			}
		case services.InstallRunningSetup:
			if lastStatus != p.Status {
				fmt.Println(styles.StepPrefix.Render("  Running Battle.net installer..."))
			}
		case services.InstallWaitingForClient:
			if lastStatus != p.Status {
				fmt.Println(styles.StepPrefix.Render("  Waiting for Battle.net to finish installing..."))
				fmt.Println(styles.Muted.Render("  (The installer will download ~400 MB — this takes a few minutes)"))
			}
		case services.InstallDone:
			if lastStatus != p.Status {
				fmt.Println(styles.StepPrefix.Render("  Cleaning up..."))
			}
		}
		lastStatus = p.Status
	}

	result, err := installer.Install(progressFn)
	if err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Installation failed: ")+err.Error())
		return err
	}

	fmt.Println()
	if result.Installed {
		fmt.Println(styles.Success.Render("Battle.net installed successfully!"))
		fmt.Println(styles.Muted.Render("  Prefix: " + result.PrefixPath))
		fmt.Println(styles.Muted.Render("  Run 'bnetctl launch' to start Battle.net"))
	} else {
		fmt.Println(styles.Warning.Render("Installation completed but Battle.net exe not found."))
		fmt.Println(styles.Muted.Render("  The installer may need more time. Try 'bnetctl install' again."))
	}

	log.Debug("install completed", "prefix", result.PrefixPath, "installed", result.Installed)
	return nil
}
