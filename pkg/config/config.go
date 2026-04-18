package config

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type FolderConfig struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

type SpaceConfig struct {
	ID      string         `toml:"id"`
	Name    string         `toml:"name"`
	Folders []FolderConfig `toml:"folders"`
}

type Config struct {
	WorkspaceID   string        `toml:"workspace_id"`
	WorkspaceName string        `toml:"workspace_name"`
	Spaces        []SpaceConfig `toml:"spaces"`
}

// ConfigPath returns the path to the config file, respecting XDG Base Directory spec.
// Priority:
//   1. $XDG_CONFIG_HOME/clickup-tui/config.toml
//   2. ~/.config/clickup-tui/config.toml
//   3. ~/.local/clickup-tui.toml (legacy)
func ConfigPath() (string, error) {
	// Check XDG_CONFIG_HOME
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "clickup-tui", "config.toml"), nil
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "clickup-tui", "config.toml"), nil
}

// getLegacyConfigPath returns the legacy config path for backwards compatibility
func GetLegacyConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "clickup-tui.toml"), nil
}

func SaveConfig(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadConfig() (Config, error) {
	var cfg Config

	// Try new XDG path first
	path, err := ConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err == nil {
		err = toml.Unmarshal(data, &cfg)
		return cfg, err
	}

	// Fall back to legacy path
	legacyPath, err := GetLegacyConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err = os.ReadFile(legacyPath)
	if err != nil {
		return cfg, err
	}

	err = toml.Unmarshal(data, &cfg)
	return cfg, err
}

func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}
