package icoutils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bnema/bnetctl/internal/logger"
	"github.com/bnema/bnetctl/internal/ports"
	"github.com/bnema/bnetctl/internal/ui/styles"
)

var _ ports.IconExtractorPort = (*Extractor)(nil)

// Extractor implements IconExtractorPort using the icoutils (wrestool + icotool) system tools.
type Extractor struct{}

// NewExtractor creates a new Extractor.
func NewExtractor() *Extractor {
	return &Extractor{}
}

// ExtractIcon extracts the best-quality icon from exePath and writes it to destPath.
// Returns the final icon path on success, or "applications-games" (XDG fallback) on failure.
func (e *Extractor) ExtractIcon(exePath string, destPath string) (string, error) {
	log := logger.Log

	// If icon already exists, reuse it
	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	if _, err := os.Stat(exePath); err != nil {
		log.Debug("battle.net exe not found, using fallback icon")
		return "applications-games", nil
	}

	// Check if icoutils is available
	wrestool, err := exec.LookPath("wrestool")
	if err != nil {
		log.Warn("wrestool not found, using fallback icon")
		fmt.Fprintln(os.Stderr, styles.Warning.Render("Icon extraction skipped: icoutils not installed"))
		fmt.Fprintln(os.Stderr, styles.Muted.Render("  Install it: sudo pacman -S icoutils"))
		return "applications-games", nil
	}
	icotool, err := exec.LookPath("icotool")
	if err != nil {
		log.Warn("icotool not found, using fallback icon")
		fmt.Fprintln(os.Stderr, styles.Warning.Render("Icon extraction skipped: icoutils not installed"))
		fmt.Fprintln(os.Stderr, styles.Muted.Render("  Install it: sudo pacman -S icoutils"))
		return "applications-games", nil
	}

	// Extract .ico from exe
	tmpIco := filepath.Join(os.TempDir(), "bnetctl-icon.ico")
	defer func() { _ = os.Remove(tmpIco) }()

	wrestoolCmd := exec.Command(wrestool, "-x", "-t", "14", "-o", tmpIco, exePath)
	if err := wrestoolCmd.Run(); err != nil {
		log.Debug("wrestool extraction failed", "error", err)
		return "applications-games", nil
	}

	tmpDir, err := os.MkdirTemp("", "bnetctl-icons-")
	if err != nil {
		return "applications-games", nil
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	icotoolCmd := exec.Command(icotool, "-x", "-o", tmpDir, tmpIco)
	if err := icotoolCmd.Run(); err != nil {
		log.Debug("icotool conversion failed", "error", err)
		return "applications-games", nil
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil || len(entries) == 0 {
		return "applications-games", nil
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
		return "applications-games", nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "applications-games", nil
	}

	data, err := os.ReadFile(bestFile)
	if err != nil {
		return "applications-games", nil
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return "applications-games", nil
	}

	log.Info("extracted battle.net icon", "path", destPath, "size", bestSize)
	return destPath, nil
}
