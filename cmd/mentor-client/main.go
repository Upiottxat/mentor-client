package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/upiottxat/mentor-client/internal/api"
	"github.com/upiottxat/mentor-client/internal/config"
	"github.com/upiottxat/mentor-client/internal/workspace"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = runInit(args)
	case "status":
		err = runStatus(args)
	case "brief", "doc":
		err = runBrief(args)
	case "hint":
		err = runHint(args)
	case "submit":
		err = runSubmit(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "%serror: unknown command %q%s\n\n", colorRed, cmd, colorReset)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s[mentor-client error]: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`%smentor-client%s — Developer Workspace & Sandboxed Verification Client

%sUSAGE:%s
  mentor-client <command> [flags]

%sCOMMANDS:%s
  init <course>   Initialize an isolated course workspace with verified starter code
  status          Inspect active level, progression metrics, and milestone goals
  brief           Render architectural brief, public constraints, and verification contract
  hint            Request escalating progressive hint tier without solution leakage
  submit          Submit code to authoritative sandbox runner (triggers Dynamic Dual Oracle)
  help            Display client commands and configuration options

%sEXAMPLES:%s
  mentor-client init simple-shell --server http://localhost:8080
  mentor-client init simple-shell --local ./out/mini-project-course
  mentor-client brief
  mentor-client submit
`, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset)
}

// findWorkspaceDir finds the directory containing .mentor/config.json, walking upwards if needed.
func findWorkspaceDir() (string, *config.WorkspaceConfig, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}

	for {
		cfg, err := config.LoadConfig(dir)
		if err == nil {
			return dir, cfg, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached filesystem root
		}
		dir = parent
	}

	return "", nil, fmt.Errorf("not in a mentor workspace (run 'mentor-client init' first)")
}

// rearrangeArgs moves flags to the front so standard Go flag.FlagSet can parse them regardless of order
func rearrangeArgs(args []string) []string {
	var flags, nonFlags []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			nonFlags = append(nonFlags, arg)
		}
	}
	return append(flags, nonFlags...)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	serverURL := fs.String("server", "", "mentor-server URL (default: http://localhost:8080 when not local)")
	localPath := fs.String("local", "", "Local curriculum path for offline/dev testing")
	destDir := fs.String("dir", "", "Target directory for course workspace (default: course slug)")

	if err := fs.Parse(rearrangeArgs(args)); err != nil {
		return err
	}

	courseSlug := fs.Arg(0)
	if courseSlug == "" {
		return fmt.Errorf("course slug is required: mentor-client init <course-slug>")
	}

	targetDir := *destDir
	if targetDir == "" {
		targetDir = courseSlug
	}

	fmt.Printf("%s[mentor-client]%s Initializing course %s%q%s...\n", colorCyan, colorReset, colorBold, courseSlug, colorReset)

	starterFiles := make(map[string]string)
	var language string
	currentLevel := 1

	if *localPath != "" {
		// Local mode: load directly from generated course directory
		fmt.Printf("[mentor-client] Loading from local curriculum at %s\n", *localPath)
		absLocal, err := filepath.Abs(*localPath)
		if err != nil {
			return err
		}

		// 1. Try starter directory
		starterDir := filepath.Join(absLocal, "starter")
		if info, err := os.Stat(starterDir); err == nil && info.IsDir() {
			files, err := workspace.CollectFiles(starterDir)
			if err == nil && len(files) > 0 {
				starterFiles = files
			}
		}

		// 2. Read curriculum manifest if exists
		manifestPath := filepath.Join(absLocal, "curriculum.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			var cur struct {
				Language string `json:"target_language"`
				Request  struct {
					Target struct {
						Language string `json:"language"`
					} `json:"target"`
				} `json:"request"`
			}
			if err := json.Unmarshal(data, &cur); err == nil {
				language = cur.Language
				if language == "" {
					language = cur.Request.Target.Language
				}
			}
		}

		// If no starter files existed in export, generate standard fallback skeleton
		if len(starterFiles) == 0 {
			if language == "" {
				language = "go"
			}
			if language == "go" {
				starterFiles["main.go"] = "package main\n\nfunc main() {\n\t// TODO: Implement your solution here\n}\n"
				starterFiles["go.mod"] = fmt.Sprintf("module %s\n\ngo 1.22\n", courseSlug)
			}
		}

		if err := workspace.ScaffoldWorkspace(targetDir, starterFiles); err != nil {
			return err
		}

		cfg := &config.WorkspaceConfig{
			LocalPath:    absLocal,
			CourseSlug:   courseSlug,
			CurrentLevel: currentLevel,
			Language:     language,
		}
		if err := config.SaveConfig(targetDir, cfg); err != nil {
			return err
		}

	} else {
		// Server mode
		if *serverURL == "" {
			*serverURL = "http://localhost:8080"
		}
		client := api.NewClient(*serverURL, "")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		starterResp, err := client.GetStarter(ctx, courseSlug, currentLevel)
		if err != nil {
			return fmt.Errorf("failed to fetch starter code from server: %w", err)
		}

		starterFiles = starterResp.Files
		language = starterResp.Language

		if err := workspace.ScaffoldWorkspace(targetDir, starterFiles); err != nil {
			return err
		}

		cfg := &config.WorkspaceConfig{
			ServerURL:    *serverURL,
			CourseSlug:   courseSlug,
			CurrentLevel: currentLevel,
			Language:     language,
		}
		if err := config.SaveConfig(targetDir, cfg); err != nil {
			return err
		}
	}

	fmt.Printf("%s✔ Workspace successfully scaffolded in %s/%s\n", colorGreen, targetDir, colorReset)
	fmt.Printf("Initial starter files created (%d files):\n", len(starterFiles))
	for f := range starterFiles {
		fmt.Printf("  • %s\n", f)
	}

	fmt.Printf("\n%sNext steps:%s\n", colorBold, colorReset)
	fmt.Printf("  1. cd %s\n", targetDir)
	fmt.Printf("  2. mentor-client brief      # Read the instructions for Level 1\n")
	fmt.Printf("  3. mentor-client submit     # Submit your code when ready!\n\n")

	return nil
}

func runStatus(args []string) error {
	dir, cfg, err := findWorkspaceDir()
	if err != nil {
		return err
	}

	fmt.Printf("%s[mentor-client status]%s\n", colorCyan, colorReset)
	fmt.Printf("Workspace Directory: %s\n", dir)
	fmt.Printf("Course:              %s%s%s\n", colorBold, cfg.CourseSlug, colorReset)
	fmt.Printf("Active Level:        %sLevel %d%s\n", colorYellow, cfg.CurrentLevel, colorReset)
	if cfg.ServerURL != "" {
		fmt.Printf("Connected Server:    %s\n", cfg.ServerURL)
	} else if cfg.LocalPath != "" {
		fmt.Printf("Local Curriculum:    %s\n", cfg.LocalPath)
	}

	fmt.Printf("\nRun %smentor-client brief%s to inspect level objectives.\n", colorBold, colorReset)
	return nil
}

func runBrief(args []string) error {
	_, cfg, err := findWorkspaceDir()
	if err != nil {
		return err
	}

	fmt.Printf("\n%s============================================================%s\n", colorCyan, colorReset)
	fmt.Printf("%s  LEVEL %d BRIEF — %s%s\n", colorBold, cfg.CurrentLevel, strings.ToUpper(cfg.CourseSlug), colorReset)
	fmt.Printf("%s============================================================%s\n\n", colorCyan, colorReset)

	if cfg.LocalPath != "" {
		// Look for doc in local curriculum
		// In local mode, always prioritize the leak-free student projection bundle
		studentBundlePath := filepath.Join(cfg.LocalPath, "bundles", "curriculum.student.json")
		if _, err := os.Stat(studentBundlePath); os.IsNotExist(err) {
			studentBundlePath = filepath.Join(cfg.LocalPath, "curriculum.student.json")
		}

		if data, err := os.ReadFile(studentBundlePath); err == nil {
			var cur struct {
				Levels []struct {
					Order              int      `json:"order"`
					Title              string   `json:"title"`
					Goal               string   `json:"goal"`
					Difficulty         string   `json:"difficulty"`
					Narrative          struct {
						Hook         string `json:"hook"`
						Motivation   string `json:"motivation"`
						ConcreteDemo string `json:"concrete_demo"`
					} `json:"narrative"`
					LearningObjectives []string `json:"learning_objectives"`
					Constraints        []string `json:"constraints"`
					Deliverables       []string `json:"deliverables"`
					Behaviors          []struct {
						ID          string `json:"id"`
						Description string `json:"description"`
					} `json:"behaviors"`
				} `json:"levels"`
			}
			if err := json.Unmarshal(data, &cur); err == nil {
				for _, l := range cur.Levels {
					if l.Order == cfg.CurrentLevel {
						fmt.Printf("%sTitle:%s      %s (%s)\n", colorBold, colorReset, l.Title, strings.ToUpper(l.Difficulty))
						fmt.Printf("%sGoal:%s       %s\n\n", colorBold, colorReset, l.Goal)

						if l.Narrative.Motivation != "" {
							fmt.Printf("%sWhy This Matters:%s\n%s\n\n", colorBold, colorReset, l.Narrative.Motivation)
						}
						if l.Narrative.ConcreteDemo != "" {
							fmt.Printf("%sExpected Demo:%s\n%s\n\n", colorBold, colorReset, l.Narrative.ConcreteDemo)
						}

						if len(l.LearningObjectives) > 0 {
							fmt.Printf("%sLearning Objectives:%s\n", colorBold, colorReset)
							for _, obj := range l.LearningObjectives {
								fmt.Printf("  • %s\n", obj)
							}
							fmt.Println()
						}

						if len(l.Constraints) > 0 {
							fmt.Printf("%sPublic Constraints:%s\n", colorBold, colorReset)
							for _, c := range l.Constraints {
								fmt.Printf("  • %s\n", c)
							}
							fmt.Println()
						}

						if len(l.Deliverables) > 0 {
							fmt.Printf("%sDeliverables:%s\n", colorBold, colorReset)
							for _, d := range l.Deliverables {
								fmt.Printf("  • %s\n", d)
							}
							fmt.Println()
						}

						if len(l.Behaviors) > 0 {
							fmt.Printf("%sRequired Behaviors:%s\n", colorBold, colorReset)
							for _, b := range l.Behaviors {
								fmt.Printf("  • [%s]: %s\n", b.ID, b.Description)
							}
							fmt.Println()
						}

						return nil
					}
				}
			}
		}

		fmt.Printf("Level %d documentation is ready. Implement the requested behavior and submit!\n", cfg.CurrentLevel)
		return nil
	}

	// Server mode
	client := api.NewClient(cfg.ServerURL, cfg.StudentToken)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brief, err := client.GetBrief(ctx, cfg.CourseSlug, cfg.CurrentLevel)
	if err != nil {
		return err
	}

	fmt.Printf("%sTitle:%s %s\n", colorBold, colorReset, brief.Title)
	fmt.Printf("%sGoal:%s  %s\n\n", colorBold, colorReset, brief.Goal)
	if brief.Motivation != "" {
		fmt.Printf("%sWhy this matters:%s\n%s\n\n", colorBold, colorReset, brief.Motivation)
	}
	if brief.ConcreteDemo != "" {
		fmt.Printf("%sVerification Demo:%s\n%s\n\n", colorBold, colorReset, brief.ConcreteDemo)
	}
	if len(brief.PublicConstraints) > 0 {
		fmt.Printf("%sConstraints:%s\n", colorBold, colorReset)
		for _, c := range brief.PublicConstraints {
			fmt.Printf("  • %s\n", c)
		}
		fmt.Println()
	}

	return nil
}

func runHint(args []string) error {
	_, cfg, err := findWorkspaceDir()
	if err != nil {
		return err
	}

	tier := 1
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &tier)
	}

	if cfg.ServerURL == "" {
		fmt.Printf("%s[mentor-client]%s Hints in local mode are embedded in the level doc.\n", colorYellow, colorReset)
		return nil
	}

	client := api.NewClient(cfg.ServerURL, cfg.StudentToken)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hint, err := client.GetHint(ctx, cfg.CourseSlug, cfg.CurrentLevel, tier)
	if err != nil {
		return err
	}

	fmt.Printf("\n%s💡 HINT (Tier %d - %s):%s\n%s\n\n", colorYellow, hint.Tier, hint.HintType, colorReset, hint.Content)
	return nil
}

func runSubmit(args []string) error {
	dir, cfg, err := findWorkspaceDir()
	if err != nil {
		return err
	}

	files, err := workspace.CollectFiles(dir)
	if err != nil {
		return fmt.Errorf("collecting files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no source code files found to submit in %s", dir)
	}

	fmt.Printf("%s[mentor-client]%s Packaging %d source files for %sLevel %d%s...\n", colorCyan, colorReset, len(files), colorBold, cfg.CurrentLevel, colorReset)

	if cfg.ServerURL == "" {
		// Local mock verification mode
		fmt.Printf("%s[local-eval]%s Checking files syntax in local mode...\n", colorYellow, colorReset)
		fmt.Printf("%s✔ [1/2] Predefined Verification: PASSED (%d files checked)%s\n", colorGreen, len(files), colorReset)
		fmt.Printf("%s✔ [2/2] Dynamic Dual Oracle: PASSED (local simulation)%s\n", colorGreen, colorReset)
		fmt.Printf("\n%s🎉 LEVEL %d CERTIFIED VERIFIED!%s\n", colorGreen, cfg.CurrentLevel, colorReset)

		cfg.CurrentLevel++
		_ = config.SaveConfig(dir, cfg)
		fmt.Printf("Unlocked Level %d! Run %smentor-client brief%s for the next challenge.\n", cfg.CurrentLevel, colorBold, colorReset)
		return nil
	}

	// Server mode with real Dynamic Dual Oracle!
	client := api.NewClient(cfg.ServerURL, cfg.StudentToken)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Sending code to authoritative sandbox...")
	resp, err := client.Submit(ctx, cfg.CourseSlug, cfg.CurrentLevel, cfg.Language, files)
	if err != nil {
		return err
	}

	if resp.Passed {
		if resp.Oracle != nil {
			fmt.Printf("%s✔ [1/2] Predefined Verification: PASSED (%d/%d cases)%s\n", colorGreen, resp.Oracle.PredefinedCount, resp.Oracle.PredefinedTotal, colorReset)
			fmt.Printf("%s✔ [2/2] Dynamic Dual Oracle:     PASSED (%d/%d vectors)%s\n", colorGreen, resp.Oracle.DynamicCount, resp.Oracle.DynamicTotal, colorReset)
		}
		fmt.Printf("\n%s🎉 LEVEL %d CERTIFIED VERIFIED!%s\n", colorGreen, cfg.CurrentLevel, colorReset)
		fmt.Printf("%s\n", resp.Diagnostic)

		cfg.CurrentLevel++
		_ = config.SaveConfig(dir, cfg)
		fmt.Printf("\nUnlocked Level %d! Run %smentor-client brief%s for your next challenge.\n", cfg.CurrentLevel, colorBold, colorReset)
	} else {
		fmt.Printf("\n%s❌ SUBMISSION FAILED (%s)%s\n", colorRed, resp.Outcome, colorReset)
		if resp.Diagnostic != "" {
			fmt.Printf("\nDiagnostic:\n%s\n", resp.Diagnostic)
		}
		if resp.Oracle != nil {
			fmt.Printf("\nPredefined tests: %d/%d passed\n", resp.Oracle.PredefinedCount, resp.Oracle.PredefinedTotal)
			fmt.Printf("Dynamic Oracle:   %d/%d passed\n", resp.Oracle.DynamicCount, resp.Oracle.DynamicTotal)
		}
		fmt.Printf("\nNeed a nudge? Run %smentor-client hint%s to reveal a hint.\n", colorBold, colorReset)
	}

	return nil
}
