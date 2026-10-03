package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the user's choices, kept in the plugin's config dir.
type Settings struct {
	AutoTab  bool `json:"auto_tab"` // keep a Council tab in every workspace (default on)
	Welcomed bool `json:"welcomed"` // the first-run screen has been shown
}

func configDir() string {
	if d := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); d != "" {
		return d
	}
	return filepath.Join(stateDir(), "config")
}

func settingsPath() string { return filepath.Join(configDir(), "settings.json") }

func loadSettings() Settings {
	s := Settings{AutoTab: true}
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(s Settings) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return writeAtomic(settingsPath(), b)
}
