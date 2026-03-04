package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var killAll bool

var killCmd = &cobra.Command{
	Use:     "kill",
	Aliases: []string{"stop"},
	Short:   "Kill all Battle.net/Wine processes",
	Long: `Stop all running Battle.net and Wine processes for the bnetctl prefix.

Use --all to also kill orphaned Wine system processes (explorer.exe, services.exe, etc.)
that may have survived previous runs.`,
	RunE: runKill,
}

func init() {
	killCmd.Flags().BoolVarP(&killAll, "all", "a", false, "Kill everything including orphaned system processes")
	rootCmd.AddCommand(killCmd)
}

func runKill(cmd *cobra.Command, args []string) error {
	log := getLogger()

	cfg, err := xdg.DefaultConfig()
	if err != nil {
		log.Error("load config failed", "error", err)
		return fmt.Errorf("load config: %w", err)
	}

	runtime := wine.NewAdapter()
	log.Info("killing wine processes", "prefix", cfg.PrefixDir, "all", killAll)

	// Step 1: Try graceful wineserver kill
	fmt.Println("Stopping wineserver...")
	_ = runtime.GracefulKillPrefix(cfg.PrefixDir, 3*time.Second)
	log.Info("wineserver stopped")

	if !killAll {
		fmt.Println(styles.Success.Render("Done."))
		return nil
	}

	// Step 2: --all mode — scan /proc and kill anything belonging to our prefix
	fmt.Println("Scanning for orphaned processes...")
	killed := killPrefixProcesses(cfg.PrefixDir)
	log.Info("orphan scan complete", "killed", killed)
	if killed > 0 {
		fmt.Printf(styles.Success.Render("Killed %d orphaned process(es).")+"\n", killed)
	} else {
		fmt.Println(styles.Success.Render("No orphaned processes found."))
	}

	return nil
}

// killPrefixProcesses kills all processes whose environment contains our prefix path.
// Returns the number of processes killed.
func killPrefixProcesses(prefixPath string) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}

	marker := "WINEPREFIX=" + filepath.Join(prefixPath, "pfx")
	myPid := os.Getpid()
	killed := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if pid[0] < '1' || pid[0] > '9' {
			continue
		}

		pidNum := 0
		for _, c := range pid {
			pidNum = pidNum*10 + int(c-'0')
		}
		if pidNum == myPid {
			continue
		}

		environPath := filepath.Join("/proc", pid, "environ")
		data, err := os.ReadFile(environPath)
		if err != nil {
			continue
		}

		if strings.Contains(string(data), marker) {
			cmdline, _ := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
			cmdStr := strings.ReplaceAll(string(cmdline), "\x00", " ")
			proc, err := os.FindProcess(pidNum)
			if err == nil {
				if err := proc.Signal(syscall.SIGKILL); err == nil {
					fmt.Printf("  killed PID %d: %s\n", pidNum, strings.TrimSpace(cmdStr))
					killed++
				}
			}
		}
	}

	return killed
}
