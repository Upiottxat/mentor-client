package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

var ignoredDirs = map[string]bool{
	".mentor":      true,
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".cache":       true,
	".idea":        true,
	".vscode":      true,
}

var ignoredExts = map[string]bool{
	".exe":   true,
	".bin":   true,
	".o":     true,
	".so":    true,
	".a":     true,
	".dll":   true,
	".dylib": true,
	".class": true,
	".pyc":   true,
}

// CollectFiles walks the workspace directory and collects source code files into a map.
func CollectFiles(dir string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		parts := strings.Split(rel, string(filepath.Separator))
		if ignoredDirs[parts[0]] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(rel))
		if ignoredExts[ext] {
			return nil
		}

		// Skip hidden files in root like .DS_Store
		if strings.HasPrefix(parts[len(parts)-1], ".") && parts[len(parts)-1] != ".gitignore" {
			return nil
		}

		// Avoid reading huge files (cap at 1MB per source file)
		if info.Size() > 1024*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})

	return files, err
}
