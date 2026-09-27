package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScaffoldAndCollectFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mentor_workspace_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	starterFiles := map[string]string{
		"main.go":           "package main\n\nfunc main() {}\n",
		"go.mod":            "module example.com/shell\n\ngo 1.22\n",
		"pkg/util/hello.go": "package util\n",
	}

	if err := ScaffoldWorkspace(tmpDir, starterFiles); err != nil {
		t.Fatalf("ScaffoldWorkspace failed: %v", err)
	}

	// Verify files written
	for relPath := range starterFiles {
		if _, err := os.Stat(filepath.Join(tmpDir, relPath)); err != nil {
			t.Errorf("expected file %s to exist: %v", relPath, err)
		}
	}

	// Verify default .gitignore written
	if _, err := os.Stat(filepath.Join(tmpDir, ".gitignore")); err != nil {
		t.Errorf("expected default .gitignore to exist: %v", err)
	}

	// Create a dummy .mentor/config.json and binary to test collector ignores
	_ = os.MkdirAll(filepath.Join(tmpDir, ".mentor"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".mentor", "config.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "output.bin"), []byte{0x00, 0x01}, 0644)

	collected, err := CollectFiles(tmpDir)
	if err != nil {
		t.Fatalf("CollectFiles failed: %v", err)
	}

	// Should contain starterFiles + .gitignore, but NOT .mentor or .bin
	if _, found := collected[".mentor/config.json"]; found {
		t.Errorf(".mentor/config.json should NOT be collected")
	}
	if _, found := collected["output.bin"]; found {
		t.Errorf("binary output.bin should NOT be collected")
	}
	if _, found := collected["main.go"]; !found {
		t.Errorf("main.go should be collected")
	}
	if _, found := collected["pkg/util/hello.go"]; !found {
		t.Errorf("pkg/util/hello.go should be collected")
	}
}
