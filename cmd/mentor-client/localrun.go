package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// expandShellVars resolves simple shell-style variables such as $(nproc) that
// generators write into make build commands.
func expandShellVars(args []string) []string {
	out := make([]string, len(args))
	const marker = "$(nproc)"
	for i, a := range args {
		if strings.Contains(a, marker) {
			a = strings.ReplaceAll(a, marker, strconv.Itoa(runtime.NumCPU()))
		}
		out[i] = a
	}
	return out
}

// runStudentProgram builds (when required) and runs the student's program in
// workspaceDir for the given language, returning captured stdout+stderr. The
// run_command/build_command from the level curriculum, when present, override
// the defaults.
func runStudentProgram(ctx context.Context, workspaceDir, language, stdin string, runCommand, buildCommand []string) (string, error) {
	lang := strings.ToLower(language)
	buildCommand = expandShellVars(buildCommand)
	runCommand = expandShellVars(runCommand)

	switch lang {
	case "cpp", "c++", "c++17", "c++20", "cxx", "cc", "c":
		buildArgs := buildCommand
		if len(buildArgs) == 0 {
			buildArgs = []string{"make"}
		}
		bc := exec.CommandContext(ctx, buildArgs[0], buildArgs[1:]...)
		bc.Dir = workspaceDir
		if out, err := bc.CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("build failed (%s): %w", strings.Join(buildArgs, " "), err)
		}
	}

	var cmd *exec.Cmd
	switch lang {
	case "python", "py", "python3":
		cmd = exec.CommandContext(ctx, "python3", "main.py")
	case "rust":
		cmd = exec.CommandContext(ctx, "cargo", "run", "--quiet")
	case "cpp", "c++", "c++17", "c++20", "cxx", "cc", "c":
		target := ""
		if len(runCommand) > 0 && strings.TrimSpace(runCommand[0]) != "" {
			target = strings.TrimSpace(runCommand[0])
		}
		if target == "" {
			target = cppBinaryTarget(workspaceDir)
		}
		if target == "" {
			return "", fmt.Errorf("could not determine the built binary to run (check build_command/run_command or Makefile TARGET)")
		}
		args := []string{target}
		if len(runCommand) > 1 {
			args = append(args, runCommand[1:]...)
		}
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	default: // go
		cmd = exec.CommandContext(ctx, "go", "run", ".")
	}

	cmd.Dir = workspaceDir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// cppBinaryTarget resolves the built executable for a C/C++ project: it honors
// a Makefile TARGET and otherwise scans build/ (then the project root) for an
// executable file. Returns an absolute path or a ./relative path.
func cppBinaryTarget(workspaceDir string) string {
	if data, err := os.ReadFile(filepath.Join(workspaceDir, "Makefile")); err == nil {
		if t := parseMakeTarget(data); t != "" {
			p := filepath.Join(workspaceDir, filepath.FromSlash(t))
			if strings.HasPrefix(p, workspaceDir) {
				return p
			}
		}
	}

	for _, dir := range []string{"build", "."} {
		matches, err := filepath.Glob(filepath.Join(workspaceDir, dir, "*"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			switch strings.ToLower(filepath.Ext(m)) {
			case ".o", ".obj", ".out", ".mod", ".c", ".cpp", ".cc", ".h", ".hpp":
				continue
			}
			if info.Mode()&0111 != 0 {
				rel, _ := filepath.Rel(workspaceDir, m)
				return filepath.Join(".", filepath.ToSlash(rel))
			}
		}
	}
	return ""
}

// parseMakeTarget extracts the value assigned to TARGET in a Makefile
// (first occurrence wins).
func parseMakeTarget(data []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "TARGET") {
			continue
		}
		if line == "TARGET" {
			continue
		}
		if idx := strings.Index(line, ":="); idx >= 0 {
			return strings.TrimSpace(line[idx+2:])
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			return strings.TrimSpace(line[idx+1:])
		}
		return strings.TrimSpace(line[len("TARGET"):])
	}
	return ""
}
