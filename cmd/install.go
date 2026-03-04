package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/http"
	"github.com/bnema/bnetctl/internal/adapters/icoutils"
	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
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

	runtime := wine.NewAdapter(getLogger())
	downloader := http.NewDownloader()
	fs := xdg.NewFilesystem()

	installer := services.NewInstallerService(runtime, downloader, fs, cfg, getLogger())

	// Check if already installed
	inst := installer.GetInstallation()
	if inst.Installed {
		log.Info("battle.net already installed", "prefix", inst.PrefixPath)
		fmt.Println(styles.Warning.Render("Battle.net is already installed."))
		return nil
	}

	log.Info("starting installation")

	// Track last status to avoid repeating messages
	var lastStatus services.InstallStatus = -1
	var detectedRuntime *domain.WineRuntime

	progressFn := func(p services.InstallProgress) {
		if detectedRuntime == nil && p.Runtime != nil {
			detectedRuntime = p.Runtime
			log.Info("wine detected", "version", detectedRuntime.Version, "ntsync", detectedRuntime.HasNTSync, "dxvk_setup", detectedRuntime.HasDXVKSetup)
			fmt.Println(styles.StepPrefix.Render("Wine: ") + detectedRuntime.Version)
			if detectedRuntime.HasNTSync {
				fmt.Println(styles.Muted.Render("  NTSync: enabled"))
			}
			if detectedRuntime.HasDXVKSetup {
				fmt.Println(styles.Muted.Render("  DXVK: will auto-install"))
			}
			if detectedRuntime.HasVKD3DSetup {
				fmt.Println(styles.Muted.Render("  VKD3D-proton: will auto-install"))
			}
		}
		switch p.Status {
		case services.InstallDetecting:
			// Wine info already displayed above; nothing else to render
		case services.InstallDownloading:
			if lastStatus != p.Status {
				log.Info("download started")
			}
			if p.Download != nil {
				if p.Download.TotalBytes > 0 {
					mb := float64(p.Download.BytesDownloaded) / 1024 / 1024
					totalMb := float64(p.Download.TotalBytes) / 1024 / 1024
					_, _ = fmt.Fprintf(os.Stdout, "\r  Downloading: %.1f / %.1f MB (%.0f%%)", mb, totalMb, p.Download.Percent)
				} else {
					mb := float64(p.Download.BytesDownloaded) / 1024 / 1024
					_, _ = fmt.Fprintf(os.Stdout, "\r  Downloading: %.1f MB", mb)
				}
			}
		case services.InstallCreatingPrefix:
			if lastStatus != p.Status {
				log.Info("creating wine prefix")
				fmt.Println()
				fmt.Println(styles.StepPrefix.Render("  Creating Wine prefix..."))
			}
		case services.InstallRunningSetup:
			if lastStatus != p.Status {
				log.Info("running installer setup")
				fmt.Println(styles.StepPrefix.Render("  Running Battle.net installer..."))
			}
		case services.InstallWaitingForClient:
			if lastStatus != p.Status {
				log.Info("waiting for battle.net client installation")
				fmt.Println(styles.StepPrefix.Render("  Waiting for Battle.net to finish installing..."))
				fmt.Println(styles.Muted.Render("  (The installer will download ~400 MB — this takes a few minutes)"))
			}
		case services.InstallDone:
			if lastStatus != p.Status {
				log.Info("installation cleanup started")
				fmt.Println(styles.StepPrefix.Render("  Cleaning up..."))
			}
		}
		lastStatus = p.Status
	}

	result, err := installer.Install(progressFn)
	if err != nil {
		log.Error("installation failed", "error", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, styles.Error.Render("Installation failed: ")+err.Error())
		return err
	}

	fmt.Println()
	if result.Installed {
		fmt.Println(styles.Success.Render("Battle.net installed successfully!"))
		fmt.Println(styles.Muted.Render("  Prefix: " + result.PrefixPath))

		// Auto-create desktop entry
		desktop := xdg.NewDesktop()
		if !desktop.EntryExists("bnetctl") {
			extractor := icoutils.NewExtractor(getLogger())
			exePath := filepath.Join(cfg.PrefixDir, "pfx", domain.BattleNetExeRelPath)
			destPath := filepath.Join(cfg.DataDir, "battlenet.png")
			iconPath := extractor.ExtractIcon(exePath, destPath)
			if err := createDesktopEntry(desktop, iconPath); err != nil {
				log.Warn("failed to create desktop entry", "error", err)
			} else {
				log.Info("desktop entry auto-created", "icon", iconPath)
				fmt.Println(styles.Muted.Render("  Desktop entry created"))
			}
		}

		fmt.Println(styles.Muted.Render("  Run 'bnetctl launch' to start Battle.net"))
	} else {
		fmt.Println(styles.Warning.Render("Installation completed but Battle.net exe not found."))
		fmt.Println(styles.Muted.Render("  The installer may need more time. Try 'bnetctl install' again."))
	}

	log.Info("installation completed", "prefix", result.PrefixPath, "installed", result.Installed)
	return nil
}
