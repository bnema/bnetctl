package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const battleNetConfigContent = `{"Client":{"HardwareAcceleration":"false","Sound":{"Enabled":"true"},"GameLaunchWindowBehavior":"2","Streaming":"false"}}`

// EnsureBattleNetConfig writes a default Battle.net.config if it is missing.
func EnsureBattleNetConfig(prefixDir string) error {
	configPath := filepath.Join(
		prefixDir,
		"pfx",
		"drive_c",
		"users",
		"steamuser",
		"AppData",
		"Roaming",
		"Battle.net",
		"Battle.net.config",
	)

	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check Battle.net config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create Battle.net config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(battleNetConfigContent), 0o644); err != nil {
		return fmt.Errorf("write Battle.net config: %w", err)
	}

	return nil
}
