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

type Config struct {
	WorkspaceID   string         `toml:"workspace_id"`
	WorkspaceName string         `toml:"workspace_name"`
	SpaceID       string         `toml:"space_id"`
	SpaceName     string         `toml:"space_name"`
	Folders       []FolderConfig `toml:"folders"`
}

func SaveConfig(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".local")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}

	path := filepath.Join(dir, "clickup-tui.toml")
	
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadConfig() (Config, error) {
	var cfg Config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}

	path := filepath.Join(home, ".local", "clickup-tui.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	err = toml.Unmarshal(data, &cfg)
	return cfg, err
}

func IsNotExist(err error) bool {
	return os.IsNotExist(err)
}
