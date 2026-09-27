package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
  init            Initialize an isolated course workspace with verified starter code
  status          Inspect active level, progression metrics, and milestone goals
  brief           Render architectural brief, public constraints, and verification contract
  hint            Request escalating progressive hint tier without solution leakage
  submit          Submit code to authoritative sandbox runner (triggers Dynamic Dual Oracle)
  help            Display client commands and configuration options

%sEXAMPLES:%s
  # Initialize workspace by specifying course directory:
  mentor-client init --course /path/to/mini-project-course [--dir ./my-shell]

  # Read active level brief:
  mentor-client brief --course /path/to/mini-project-course [--level 1]
  # (or run 'mentor-client brief' inside your workspace)

  # Submit and test your solution:
  mentor-client submit --course /path/to/mini-project-course
  # (or run 'mentor-client submit' inside your workspace)
`, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset)
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

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	courseFlag := fs.String("course", "", "Path to local course directory (or course slug in server mode)")
	localFlag := fs.String("local", "", "Alias for --course")
	serverURL := fs.String("server", "", "mentor-server URL (default: http://localhost:8080 when using server)")
	destDir := fs.String("dir", "", "Target directory for course workspace (default: course slug)")

	if err := fs.Parse(rearrangeArgs(args)); err != nil {
		return err
	}

	courseInput := *courseFlag
	if courseInput == "" {
		courseInput = *localFlag
	}
	if courseInput == "" && fs.NArg() > 0 {
		courseInput = fs.Arg(0)
	}

	if courseInput == "" {
		return fmt.Errorf("course directory is required: mentor-client init --course <path-to-course> [--dir <target>]")
	}

	starterFiles := make(map[string]string)
	var courseSlug string
	var language string
	currentLevel := 1

	// Check if courseInput points to a local directory on disk
	absCourse, err := filepath.Abs(courseInput)
	isLocalDir := false
	if err == nil {
		if info, err := os.Stat(absCourse); err == nil && info.IsDir() {
			isLocalDir = true
		}
	}

	if isLocalDir {
		// Local mode: directly read from the course directory specified from terminal!
		fmt.Printf("%s[mentor-client]%s Reading course from: %s%s%s\n", colorCyan, colorReset, colorBold, absCourse, colorReset)

		// 1. Try starter directory
		starterDir := filepath.Join(absCourse, "starter")
		if info, err := os.Stat(starterDir); err == nil && info.IsDir() {
			files, err := workspace.CollectFiles(starterDir)
			if err == nil && len(files) > 0 {
				starterFiles = files
			}
		}

		// 2. Read curriculum manifest to auto-detect course slug and language
		manifestPath := filepath.Join(absCourse, "curriculum.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			var cur struct {
				CourseSlug string `json:"course_slug"`
				Slug       string `json:"slug"`
				Language   string `json:"target_language"`
				Request    struct {
					Project struct {
						Slug string `json:"slug"`
					} `json:"project"`
					Target struct {
						Language string `json:"language"`
					} `json:"target"`
				} `json:"request"`
			}
			if err := json.Unmarshal(data, &cur); err == nil {
				courseSlug = cur.CourseSlug
				if courseSlug == "" {
					courseSlug = cur.Slug
				}
				if courseSlug == "" {
					courseSlug = cur.Request.Project.Slug
				}

				language = cur.Language
				if language == "" {
					language = cur.Request.Target.Language
				}
			}
		}

		if courseSlug == "" {
			courseSlug = filepath.Base(absCourse)
		}
		if language == "" {
			language = "go"
		}

		// Fallback skeleton if starter dir was empty
		if len(starterFiles) == 0 {
			if language == "go" {
				starterFiles["main.go"] = "package main\n\nfunc main() {\n\t// TODO: Implement your solution here\n}\n"
				starterFiles["go.mod"] = fmt.Sprintf("module %s\n\ngo 1.22\n", courseSlug)
			}
		}

		targetDir := *destDir
		if targetDir == "" {
			targetDir = courseSlug
		}

		if err := workspace.ScaffoldWorkspace(targetDir, starterFiles); err != nil {
			return err
		}

		cfg := &config.WorkspaceConfig{
			LocalPath:    absCourse,
			CourseSlug:   courseSlug,
			CurrentLevel: currentLevel,
			Language:     language,
		}
		if err := config.SaveConfig(targetDir, cfg); err != nil {
			return err
		}

		fmt.Printf("%s✔ Workspace successfully scaffolded in %s/%s\n", colorGreen, targetDir, colorReset)
		fmt.Printf("Course Slug:   %s\n", courseSlug)
		fmt.Printf("Language:      %s\n", language)
		fmt.Printf("Starter Files: %d files created\n", len(starterFiles))
		for f := range starterFiles {
			fmt.Printf("  • %s\n", f)
		}

		fmt.Printf("\n%sNext steps:%s\n", colorBold, colorReset)
		fmt.Printf("  1. cd %s\n", targetDir)
		fmt.Printf("  2. mentor-client brief      # Inspect level 1 requirements\n")
		fmt.Printf("  3. mentor-client submit     # Test your solution!\n\n")
		return nil
	}

	// Server mode
	courseSlug = courseInput
	targetDir := *destDir
	if targetDir == "" {
		targetDir = courseSlug
	}

	srv := *serverURL
	if srv == "" {
		srv = "http://localhost:8080"
	}

	client := api.NewClient(srv, "")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Printf("%s[mentor-client]%s Fetching starter from server %s for %q...\n", colorCyan, colorReset, srv, courseSlug)
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
		ServerURL:    srv,
		CourseSlug:   courseSlug,
		CurrentLevel: currentLevel,
		Language:     language,
	}
	if err := config.SaveConfig(targetDir, cfg); err != nil {
		return err
	}

	fmt.Printf("%s✔ Workspace scaffolded in %s/%s\n", colorGreen, targetDir, colorReset)
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	courseFlag := fs.String("course", "", "Path to local course directory")
	if err := fs.Parse(rearrangeArgs(args)); err != nil {
		return err
	}

	var coursePath string
	var courseSlug string
	var currentLevel int
	var workspaceDir string

	dir, cfg, err := findWorkspaceDir()
	if err == nil {
		workspaceDir = dir
		coursePath = cfg.LocalPath
		courseSlug = cfg.CourseSlug
		currentLevel = cfg.CurrentLevel
	}

	if *courseFlag != "" {
		abs, err := filepath.Abs(*courseFlag)
		if err == nil {
			coursePath = abs
			courseSlug = filepath.Base(abs)
			if currentLevel == 0 {
				currentLevel = 1
			}
		}
	}

	if coursePath == "" && courseSlug == "" {
		return fmt.Errorf("not in a workspace and no --course specified (run with --course <path> or inside a workspace)")
	}

	fmt.Printf("%s[mentor-client status]%s\n", colorCyan, colorReset)
	if workspaceDir != "" {
		fmt.Printf("Workspace Directory: %s\n", workspaceDir)
	}
	fmt.Printf("Course:              %s%s%s\n", colorBold, courseSlug, colorReset)
	fmt.Printf("Active Level:        %sLevel %d%s\n", colorYellow, currentLevel, colorReset)
	if coursePath != "" {
		fmt.Printf("Curriculum Source:   %s\n", coursePath)
	}

	fmt.Printf("\nRun %smentor-client brief%s to inspect level objectives.\n", colorBold, colorReset)
	return nil
}

func runBrief(args []string) error {
	fs := flag.NewFlagSet("brief", flag.ExitOnError)
	courseFlag := fs.String("course", "", "Path to course curriculum directory")
	levelFlag := fs.Int("level", 0, "Specific level number to inspect")

	if err := fs.Parse(rearrangeArgs(args)); err != nil {
		return err
	}

	var coursePath string
	var courseSlug string
	var currentLevel int
	var serverURL string
	var token string

	_, cfg, err := findWorkspaceDir()
	if err == nil {
		coursePath = cfg.LocalPath
		courseSlug = cfg.CourseSlug
		currentLevel = cfg.CurrentLevel
		serverURL = cfg.ServerURL
		token = cfg.StudentToken
	}

	if *courseFlag != "" {
		abs, err := filepath.Abs(*courseFlag)
		if err == nil {
			coursePath = abs
			courseSlug = filepath.Base(abs)
		}
	}

	if *levelFlag > 0 {
		currentLevel = *levelFlag
	} else if currentLevel == 0 {
		currentLevel = 1
	}

	if coursePath == "" && serverURL == "" {
		return fmt.Errorf("no course specified: provide --course <path> or run inside a mentor workspace")
	}

	fmt.Printf("\n%s============================================================%s\n", colorCyan, colorReset)
	fmt.Printf("%s  LEVEL %d BRIEF — %s%s\n", colorBold, currentLevel, strings.ToUpper(courseSlug), colorReset)
	fmt.Printf("%s============================================================%s\n\n", colorCyan, colorReset)

	if coursePath != "" {
		// Read directly from coursePath
		studentBundlePath := filepath.Join(coursePath, "bundles", "curriculum.student.json")
		if _, err := os.Stat(studentBundlePath); os.IsNotExist(err) {
			studentBundlePath = filepath.Join(coursePath, "curriculum.student.json")
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
					if l.Order == currentLevel {
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

		fmt.Printf("Level %d brief is ready in %s. Implement the requested behavior and submit!\n", currentLevel, coursePath)
		return nil
	}

	// Server mode
	client := api.NewClient(serverURL, token)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brief, err := client.GetBrief(ctx, courseSlug, currentLevel)
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
	tier := 1
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &tier)
	}

	_, cfg, err := findWorkspaceDir()
	if err != nil || cfg.ServerURL == "" {
		fmt.Printf("%s[mentor-client]%s In local mode, conceptual guidance is detailed in the level brief.\n", colorYellow, colorReset)
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
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	courseFlag := fs.String("course", "", "Path to course curriculum directory")
	levelFlag := fs.Int("level", 0, "Specific level number to verify against")
	dirFlag := fs.String("dir", "", "Target directory containing student code to submit (default: current directory)")

	if err := fs.Parse(rearrangeArgs(args)); err != nil {
		return err
	}

	var coursePath string
	var currentLevel int
	var serverURL string
	var token string
	var language string
	var workspaceDir string

	dir, cfg, err := findWorkspaceDir()
	if err == nil {
		workspaceDir = dir
		coursePath = cfg.LocalPath
		currentLevel = cfg.CurrentLevel
		serverURL = cfg.ServerURL
		token = cfg.StudentToken
		language = cfg.Language
	}

	if *dirFlag != "" {
		workspaceDir = *dirFlag
	} else if workspaceDir == "" {
		workspaceDir = "."
	}

	if *courseFlag != "" {
		abs, err := filepath.Abs(*courseFlag)
		if err == nil {
			coursePath = abs
		}
	}

	if *levelFlag > 0 {
		currentLevel = *levelFlag
	} else if currentLevel == 0 {
		currentLevel = 1
	}

	if coursePath == "" && serverURL == "" {
		return fmt.Errorf("no course source found: run inside workspace or specify --course <path>")
	}

	files, err := workspace.CollectFiles(workspaceDir)
	if err != nil {
		return fmt.Errorf("collecting files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no source code files found to submit in %s", workspaceDir)
	}

	fmt.Printf("%s[mentor-client]%s Packaging %d source files for %sLevel %d%s...\n", colorCyan, colorReset, len(files), colorBold, currentLevel, colorReset)

	if serverURL == "" {
		// Real local verification mode
		fmt.Printf("%s[local-eval]%s Executing test verification in local mode...\n", colorYellow, colorReset)

		type SpecCase struct {
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			RunOn    string `json:"run_on"`
			Stdin    string `json:"stdin"`
			Expected string `json:"expected"`
			Matcher  string `json:"matcher"`
		}

		var cases []SpecCase

		// 1. Locate spec for current level
		specFile := filepath.Join(coursePath, "server", "specs", fmt.Sprintf("level-%02d.spec.json", currentLevel))
		if _, err := os.Stat(specFile); os.IsNotExist(err) {
			specFile = filepath.Join(coursePath, "server", "specs", fmt.Sprintf("level-%d.spec.json", currentLevel))
		}

		if data, err := os.ReadFile(specFile); err == nil {
			var specData struct {
				AgnosticTestSpec struct {
					Cases []SpecCase `json:"cases"`
				} `json:"agnostic_test_spec"`
			}
			if err := json.Unmarshal(data, &specData); err == nil {
				cases = specData.AgnosticTestSpec.Cases
			}
		}

		if len(cases) == 0 {
			// Fallback: look in curriculum.json
			curPath := filepath.Join(coursePath, "curriculum.json")
			if data, err := os.ReadFile(curPath); err == nil {
				var cur struct {
					Levels []struct {
						Order            int `json:"order"`
						AgnosticTestSpec struct {
							Cases []SpecCase `json:"cases"`
						} `json:"agnostic_test_spec"`
					} `json:"levels"`
				}
				if err := json.Unmarshal(data, &cur); err == nil {
					for _, l := range cur.Levels {
						if l.Order == currentLevel {
							cases = l.AgnosticTestSpec.Cases
							break
						}
					}
				}
			}
		}

		if len(cases) == 0 {
			return fmt.Errorf("no test cases found for level %d in local curriculum at %s", currentLevel, coursePath)
		}

		predefinedPassed, predefinedTotal := 0, 0
		dynamicPassed, dynamicTotal := 0, 0
		allPassed := true
		var failureMsg string

		for _, tc := range cases {
			isDynamic := (tc.Kind == "dynamic_oracle" || tc.RunOn == "submit_only")
			if isDynamic {
				dynamicTotal++
			} else {
				predefinedTotal++
			}

			// Run code locally in student workspace dir
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var cmd *exec.Cmd
			switch strings.ToLower(language) {
			case "python", "py", "python3":
				cmd = exec.CommandContext(ctx, "python3", "main.py")
			case "rust":
				cmd = exec.CommandContext(ctx, "cargo", "run", "--quiet")
			default: // go
				cmd = exec.CommandContext(ctx, "go", "run", ".")
			}
			cmd.Dir = workspaceDir
			if tc.Stdin != "" {
				cmd.Stdin = strings.NewReader(tc.Stdin)
			}

			outBytes, err := cmd.CombinedOutput()
			cancel()

			out := string(outBytes)

			if err != nil {
				allPassed = false
				failureMsg = fmt.Sprintf("Execution error on case %q: %v\nOutput:\n%s", tc.Name, err, out)
				break
			}

			matched := false
			switch tc.Matcher {
			case "trim":
				matched = (strings.TrimSpace(out) == strings.TrimSpace(tc.Expected))
			case "contains":
				matched = strings.Contains(out, tc.Expected)
			default: // exact
				matched = (out == tc.Expected)
			}

			if !matched {
				allPassed = false
				failureMsg = fmt.Sprintf("Assertion failed on case %q (matcher: %s):\n  Expected: %q\n  Actual:   %q", tc.Name, tc.Matcher, tc.Expected, out)
				break
			}

			if isDynamic {
				dynamicPassed++
			} else {
				predefinedPassed++
			}
		}

		if !allPassed {
			fmt.Printf("\n%s❌ SUBMISSION FAILED%s\n\n%s\n", colorRed, colorReset, failureMsg)
			if predefinedTotal > 0 {
				fmt.Printf("\nPredefined tests: %d/%d passed\n", predefinedPassed, predefinedTotal)
			}
			if dynamicTotal > 0 {
				fmt.Printf("Dynamic Oracle:   %d/%d passed\n", dynamicPassed, dynamicTotal)
			}
			fmt.Printf("\nNeed a nudge? Run %smentor-client hint%s to reveal a hint.\n", colorBold, colorReset)
			return nil
		}

		fmt.Printf("%s✔ [1/2] Predefined Verification: PASSED (%d/%d cases)%s\n", colorGreen, predefinedPassed, predefinedTotal, colorReset)
		if dynamicTotal > 0 {
			fmt.Printf("%s✔ [2/2] Dynamic Dual Oracle:     PASSED (%d/%d vectors)%s\n", colorGreen, dynamicPassed, dynamicTotal, colorReset)
		}
		fmt.Printf("\n%s🎉 LEVEL %d CERTIFIED VERIFIED!%s\n", colorGreen, currentLevel, colorReset)

		if cfg != nil {
			cfg.CurrentLevel++
			_ = config.SaveConfig(workspaceDir, cfg)
			fmt.Printf("Unlocked Level %d! Run %smentor-client brief%s for the next challenge.\n", cfg.CurrentLevel, colorBold, colorReset)
		}
		return nil
	}

	// Server mode
	client := api.NewClient(serverURL, token)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("Sending code to authoritative sandbox...")
	resp, err := client.Submit(ctx, cfg.CourseSlug, currentLevel, language, files)
	if err != nil {
		return err
	}

	if resp.Passed {
		if resp.Oracle != nil {
			fmt.Printf("%s✔ [1/2] Predefined Verification: PASSED (%d/%d cases)%s\n", colorGreen, resp.Oracle.PredefinedCount, resp.Oracle.PredefinedTotal, colorReset)
			fmt.Printf("%s✔ [2/2] Dynamic Dual Oracle:     PASSED (%d/%d vectors)%s\n", colorGreen, resp.Oracle.DynamicCount, resp.Oracle.DynamicTotal, colorReset)
		}
		fmt.Printf("\n%s🎉 LEVEL %d CERTIFIED VERIFIED!%s\n", colorGreen, currentLevel, colorReset)
		fmt.Printf("%s\n", resp.Diagnostic)

		if cfg != nil {
			cfg.CurrentLevel++
			_ = config.SaveConfig(workspaceDir, cfg)
			fmt.Printf("\nUnlocked Level %d! Run %smentor-client brief%s for your next challenge.\n", cfg.CurrentLevel, colorBold, colorReset)
		}
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
