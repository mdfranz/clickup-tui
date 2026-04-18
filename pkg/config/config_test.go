package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	// Use a temporary directory for testing
	tmpDir := t.TempDir()

	// Set up XDG_CONFIG_HOME to use temp directory
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() {
		if oldXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	// Create test config
	testCfg := Config{
		WorkspaceID:   "ws-123",
		WorkspaceName: "My Workspace",
		SpaceID:       "space-456",
		SpaceName:     "My Space",
		Folders: []FolderConfig{
			{ID: "folder-1", Name: "Folder 1"},
			{ID: "folder-2", Name: "Folder 2"},
		},
	}

	// Save config
	err := SaveConfig(testCfg)
	if err != nil {
		t.Fatalf("SaveConfig() failed: %v", err)
	}

	// Verify file was created
	path, _ := ConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Config file was not created at %s", path)
	}

	// Load config
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Verify config matches
	if loadedCfg.WorkspaceID != testCfg.WorkspaceID {
		t.Errorf("WorkspaceID mismatch: got %q, want %q", loadedCfg.WorkspaceID, testCfg.WorkspaceID)
	}
	if loadedCfg.WorkspaceName != testCfg.WorkspaceName {
		t.Errorf("WorkspaceName mismatch: got %q, want %q", loadedCfg.WorkspaceName, testCfg.WorkspaceName)
	}
	if loadedCfg.SpaceID != testCfg.SpaceID {
		t.Errorf("SpaceID mismatch: got %q, want %q", loadedCfg.SpaceID, testCfg.SpaceID)
	}
	if loadedCfg.SpaceName != testCfg.SpaceName {
		t.Errorf("SpaceName mismatch: got %q, want %q", loadedCfg.SpaceName, testCfg.SpaceName)
	}
	if len(loadedCfg.Folders) != len(testCfg.Folders) {
		t.Errorf("Folders count mismatch: got %d, want %d", len(loadedCfg.Folders), len(testCfg.Folders))
	}
}

func TestLoadConfig_NotExist(t *testing.T) {
	// Use a completely non-existent home directory path
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent", "config")

	// Set up XDG_CONFIG_HOME to non-existent directory
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	oldHome := os.Getenv("HOME")
	os.Setenv("XDG_CONFIG_HOME", nonExistentDir)
	os.Setenv("HOME", filepath.Join(tmpDir, "nonexistent_home"))
	defer func() {
		if oldXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
		if oldHome == "" {
			os.Unsetenv("HOME")
		} else {
			os.Setenv("HOME", oldHome)
		}
	}()

	_, err := LoadConfig()
	if err == nil {
		t.Errorf("LoadConfig() should return error for non-existent config")
	}
}

func TestXDGConfigPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with XDG_CONFIG_HOME set
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer func() {
		if oldXDG == "" {
			os.Unsetenv("XDG_CONFIG_HOME")
		} else {
			os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "clickup-tui", "config.toml")
	if path != expectedPath {
		t.Errorf("Path mismatch: got %q, want %q", path, expectedPath)
	}
}

func TestIsNotExist(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"os.IsNotExist error", os.ErrNotExist, true},
		{"nil error", nil, false},
		{"other error", os.ErrPermission, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotExist(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotExist(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}
