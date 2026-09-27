package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WorkspaceConfig records the student's active workspace state.
type WorkspaceConfig struct {
	ServerURL    string `json:"server_url,omitempty"`
	LocalPath    string `json:"local_path,omitempty"` // For local dev/testing
	CourseSlug   string `json:"course_slug"`
	StudentToken string `json:"student_token,omitempty"`
	CurrentLevel int    `json:"current_level"`
	Language     string `json:"language,omitempty"`
}

// ConfigPath returns the absolute or relative path to .mentor/config.json in the workspace.
func ConfigPath(dir string) string {
	return filepath.Join(dir, ".mentor", "config.json")
}

// LoadConfig reads and parses .mentor/config.json from the specified workspace directory.
func LoadConfig(dir string) (*WorkspaceConfig, error) {
	path := ConfigPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no mentor workspace found in %s (run 'mentor-client init' first)", dir)
	}
	var cfg WorkspaceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid .mentor/config.json: %w", err)
	}
	return &cfg, nil
}

// SaveConfig atomically writes the workspace configuration to .mentor/config.json.
func SaveConfig(dir string, cfg *WorkspaceConfig) error {
	mentorDir := filepath.Join(dir, ".mentor")
	if err := os.MkdirAll(mentorDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Atomic save via temp file + rename
	tmpFile, err := os.CreateTemp(mentorDir, ".config_*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, ConfigPath(dir))
}
