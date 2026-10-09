package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// courseExts returns the file extensions a local course artifact may use.
var courseExts = []string{".json", ".yaml", ".yml"}

// extCandidates expands one or more file paths (with or without extension)
// into the full set of JSON/YAML candidates to try, first match wins.
func extCandidates(paths ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range paths {
		base := p
		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".json", ".yaml", ".yml":
			base = strings.TrimSuffix(p, ext)
		}
		for _, e := range courseExts {
			c := base + e
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// decodeCourseData parses a local course data file into out. JSON content is
// decoded directly; YAML content is normalized through a generic map so that
// struct json tags (snake_case keys) work identically for both formats. If the
// strict YAML parse fails, a retry is attempted after expanding tabs used as
// leading indentation (some generators emit those).
func decodeCourseData(data []byte, out any) error {
	if len(data) == 0 {
		return fmt.Errorf("empty course data file")
	}
	if err := json.Unmarshal(data, out); err == nil {
		return nil
	}
	if err := decodeYAML(data, out); err == nil {
		return nil
	}
	return decodeYAML(repairYAMLIndentation(data), out)
}

func decodeYAML(data []byte, out any) error {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	jb, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(jb, out)
}

// repairYAMLIndentation expands tabs used in leading indentation to spaces so
// that otherwise-valid YAML emitted with local tab characters still parses.
// Tab characters after the first non-whitespace rune are left untouched.
func repairYAMLIndentation(data []byte) []byte {
	const tabWidth = 4
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		if !strings.ContainsRune(line, '\t') {
			continue
		}
		j := 0
		for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
			j++
		}
		if j == 0 {
			continue
		}
		var sb strings.Builder
		col := 0
		for k := 0; k < j; k++ {
			if line[k] == '\t' {
				n := tabWidth - col%tabWidth
				sb.WriteString(strings.Repeat(" ", n))
				col += n
			} else {
				sb.WriteByte(' ')
				col++
			}
		}
		sb.WriteString(line[j:])
		lines[i] = sb.String()
		changed = true
	}
	if !changed {
		return data
	}
	return []byte(strings.Join(lines, "\n"))
}

// courseReadFile returns the bytes of the first readable candidate path.
func courseReadFile(candidates []string) ([]byte, string, bool) {
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if data, err := os.ReadFile(p); err == nil {
				return data, p, true
			}
		}
	}
	return nil, "", false
}

// loadCourseFile reads and decodes the first available JSON/YAML candidate
// into out, returning the matched path.
func loadCourseFile(candidates []string, out any) (string, bool) {
	data, p, ok := courseReadFile(candidates)
	if !ok {
		return "", false
	}
	if err := decodeCourseData(data, out); err != nil {
		return p, false
	}
	return p, true
}

// looksLikeLocalfsPath reports whether an input is written like a filesystem
// path rather than a course slug (relative prefix, separator, drive letter...).
func looksLikeLocalfsPath(s string) bool {
	if strings.HasPrefix(s, ".") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") {
		return true
	}
	if strings.HasPrefix(s, "~/") {
		return true
	}
	if len(s) >= 3 && s[1] == ':' && (s[2] == '/' || s[2] == '\\') {
		return true
	}
	return strings.ContainsAny(s, "/\\")
}

// levelStarterFiles returns the level-1 starter_code files (falling back to the
// reference_solution) from the local curriculum, keyed by relative path.
func levelStarterFiles(coursePath, language string) map[string]string {
	if coursePath == "" {
		return nil
	}
	var lvl struct {
		StarterCode struct {
			Files map[string]string `json:"files"`
		} `json:"starter_code"`
		ReferenceSolution struct {
			Files map[string]string `json:"files"`
		} `json:"reference_solution"`
	}
	paths := []string{filepath.Join(coursePath, "common", "levels", "level-01")}
	if language != "" {
		paths = append([]string{filepath.Join(coursePath, "languages", language, "levels", "level-01")}, paths...)
	}
	if _, ok := loadCourseFile(extCandidates(paths...), &lvl); !ok {
		return nil
	}
	if len(lvl.StarterCode.Files) > 0 {
		return lvl.StarterCode.Files
	}
	if len(lvl.ReferenceSolution.Files) > 0 {
		return lvl.ReferenceSolution.Files
	}
	return nil
}

// classifyCourseInput decides whether a --course value names a local directory
// or a server course slug. Path-like inputs that are not on disk produce a
// clear error instead of silently falling back to the server.
func classifyCourseInput(input string) (abs string, local bool, err error) {
	if input == "" {
		return "", false, nil
	}
	abs, absErr := filepath.Abs(input)
	if absErr != nil {
		abs = input
	}
	if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
		return abs, true, nil
	}
	if looksLikeLocalfsPath(input) {
		return abs, false, fmt.Errorf(
			"course directory not found: %q (checked %s)\n"+
				"  To point at a local course use a path that exists, e.g. --course ./path/to/course.\n"+
				"  Otherwise pass a server course slug (no path separators).",
			input, abs)
	}
	return abs, false, nil
}
