package icoutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/ports"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var _ ports.IconExtractorPort = (*Extractor)(nil)

// Extractor implements IconExtractorPort using the icoutils (wrestool + icotool) system tools.
type Extractor struct {
	log *log.Logger
}

// NewExtractor creates a new Extractor.
func NewExtractor(log *log.Logger) *Extractor {
	return &Extractor{log: log}
}

// ExtractIcon extracts the best-quality icon from exePath and writes it to destPath.
// Returns the final icon path on success, or "applications-games" (XDG fallback) on failure.
func (e *Extractor) ExtractIcon(exePath string, destPath string) string {
	log := e.log

	// If icon already exists, reuse it
	if _, err := os.Stat(destPath); err == nil {
		return destPath
	}

	if _, err := os.Stat(exePath); err != nil {
		log.Debug("battle.net exe not found, using fallback icon")
		return "applications-games"
	}

	// Check if icoutils is available
	wrestool, err := exec.LookPath("wrestool")
	if err != nil {
		log.Warn("wrestool not found, using fallback icon")
		fmt.Fprintln(os.Stderr, styles.Warning.Render("Icon extraction skipped: icoutils not installed"))
		fmt.Fprintln(os.Stderr, styles.Muted.Render("  Install it: sudo pacman -S icoutils"))
		return "applications-games"
	}
	icotool, err := exec.LookPath("icotool")
	if err != nil {
		log.Warn("icotool not found, using fallback icon")
		fmt.Fprintln(os.Stderr, styles.Warning.Render("Icon extraction skipped: icoutils not installed"))
		fmt.Fprintln(os.Stderr, styles.Muted.Render("  Install it: sudo pacman -S icoutils"))
		return "applications-games"
	}

	// Extract .ico from exe
	tmpIco := filepath.Join(os.TempDir(), "bnetctl-icon.ico")
	defer func() { _ = os.Remove(tmpIco) }()

	wrestoolCmd := exec.Command(wrestool, "-x", "-t", "14", "-o", tmpIco, exePath)
	if err := wrestoolCmd.Run(); err != nil {
		log.Debug("wrestool extraction failed", "error", err)
		return "applications-games"
	}

	tmpDir, err := os.MkdirTemp("", "bnetctl-icons-")
	if err != nil {
		return "applications-games"
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	icotoolCmd := exec.Command(icotool, "-x", "-o", tmpDir, tmpIco)
	if err := icotoolCmd.Run(); err != nil {
		log.Debug("icotool conversion failed", "error", err)
		return "applications-games"
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return "applications-games"
	}

	var bestFile string
	var bestSize int64
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".png" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			bestFile = filepath.Join(tmpDir, entry.Name())
		}
	}

	if bestFile == "" {
		return "applications-games"
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "applications-games"
	}

	data, err := os.ReadFile(bestFile)
	if err != nil {
		return "applications-games"
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return "applications-games"
	}

	log.Info("extracted battle.net icon", "path", destPath, "size", bestSize)
	return destPath
}
