package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/logger"
)

// defaultBattleNetConfig is the baseline config for Battle.net under Wine.
// Do NOT set HardwareAcceleration: false — it breaks CEF rendering under Wine.
// LastLoginTassadar/LastLoginAddress are required because Battle.net's backend
// cert validation fails under Wine — these cached values provide the fallback
// login URL that lets the auth flow recover.
var defaultBattleNetConfig = map[string]any{
	"Client": map[string]any{},
	"5a61123b37cafce1": map[string]any{
		"Client": map[string]any{
			"Language": "enUS",
			"LoginSettings": map[string]any{
				"AllowedRegions": "",
				"AllowedLocales": "",
			},
		},
		"Services": map[string]any{
			"LastLoginRegion":   "US",
			"LastLoginAddress":  "us.actual.battle.net",
			"LastLoginTassadar": "account.battle.net",
		},
	},
	"Games": map[string]any{
		"battle_net": map[string]any{
			"ServerUid": "battle.net",
		},
	},
}

// EnsureBattleNetConfig ensures required config values are present.
// Creates the file if missing, or merges required keys into existing config.
func EnsureBattleNetConfig(prefixDir string) error {
	log := logger.Log

	username := domain.WineUsername()
	configPath := filepath.Join(
		prefixDir, "pfx", "drive_c", "users", username,
		"AppData", "Roaming", "Battle.net", "Battle.net.config",
	)
	log.Debug("ensuring battle.net config", "path", configPath, "username", username)

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create Battle.net config directory: %w", err)
	}

	// Load existing config or start fresh
	existing := make(map[string]any)
	data, err := os.ReadFile(configPath)
	if err == nil {
		_ = json.Unmarshal(data, &existing)
		log.Debug("loaded existing config", "keys", len(existing))
	}

	// Merge defaults into existing (defaults don't overwrite existing keys)
	merged := deepMerge(existing, defaultBattleNetConfig)

	out, err := json.MarshalIndent(merged, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal Battle.net config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0o644); err != nil {
		return fmt.Errorf("write Battle.net config: %w", err)
	}
	log.Info("battle.net config written", "path", configPath)

	return nil
}

// deepMerge merges src into dst. Values in dst are preserved; missing keys
// are filled from src. Nested maps are merged recursively.
func deepMerge(dst, src map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if existing, ok := result[k]; ok {
			// Both are maps — recurse
			if existMap, ok1 := existing.(map[string]any); ok1 {
				if srcMap, ok2 := v.(map[string]any); ok2 {
					result[k] = deepMerge(existMap, srcMap)
					continue
				}
			}
			// dst already has this key — keep it
			continue
		}
		result[k] = v
	}
	return result
}
