package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldWorkspace unpacks starter code files into the destination directory.
func ScaffoldWorkspace(destDir string, starterFiles map[string]string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	hasGitignore := false

	for relPath, content := range starterFiles {
		cleaned := filepath.Clean(relPath)
		if filepath.IsAbs(cleaned) || filepath.HasPrefix(cleaned, "..") {
			continue // Prevent path traversal
		}

		if cleaned == ".gitignore" {
			hasGitignore = true
		}

		target := filepath.Join(destDir, cleaned)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("failed to create parent dir for %s: %w", relPath, err)
		}
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write starter file %s: %w", relPath, err)
		}
	}

	// Write default .gitignore if not provided
	if !hasGitignore {
		defaultGitignore := ".mentor/\n*.exe\n*.bin\n*.o\n*.so\n"
		_ = os.WriteFile(filepath.Join(destDir, ".gitignore"), []byte(defaultGitignore), 0644)
	}

	return nil
}
