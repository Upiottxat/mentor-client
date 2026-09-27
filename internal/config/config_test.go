package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mentor_config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &WorkspaceConfig{
		ServerURL:    "http://localhost:8080",
		CourseSlug:   "simple-shell",
		StudentToken: "token_abc123",
		CurrentLevel: 1,
		Language:     "go",
	}

	if err := SaveConfig(tmpDir, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file was written
	if _, err := os.Stat(filepath.Join(tmpDir, ".mentor", "config.json")); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	loaded, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.CourseSlug != "simple-shell" {
		t.Errorf("expected course slug 'simple-shell', got %q", loaded.CourseSlug)
	}
	if loaded.CurrentLevel != 1 {
		t.Errorf("expected level 1, got %d", loaded.CurrentLevel)
	}
	if loaded.Language != "go" {
		t.Errorf("expected language 'go', got %q", loaded.Language)
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mentor_config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = LoadConfig(tmpDir)
	if err == nil {
		t.Fatalf("expected error when config not found, got nil")
	}
}
