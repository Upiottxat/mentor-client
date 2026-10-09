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
	"github.com/upiottxat/mentor-client/internal/ui"
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
	args, interactive, animation, pagination := extractGlobalFlags(os.Args[1:])
	ui.Configure(interactive, animation, pagination)

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	var err error
	switch cmd {
	case "init":
		err = runInit(cmdArgs)
	case "status":
		err = runStatus(cmdArgs)
	case "brief", "doc":
		err = runBrief(cmdArgs)
	case "hint":
		err = runHint(cmdArgs)
	case "submit":
		err = runSubmit(cmdArgs)
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

// extractGlobalFlags pulls process-wide UI flags out of the argument list so
// they work anywhere on the command line, not just after the subcommand.
func extractGlobalFlags(args []string) (remaining []string, interactive, animation, pagination bool) {
	interactive, animation, pagination = true, true, true
	for _, a := range args {
		switch a {
		case "--no-interactive", "--no-tui":
			interactive = false
		case "--no-animation", "--no-anim":
			animation = false
		case "--no-pagination", "--no-paging":
			pagination = false
		case "--plain":
			interactive, animation, pagination = false, false, false
		default:
			remaining = append(remaining, a)
		}
	}
	return remaining, interactive, animation, pagination
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

%sGLOBAL FLAGS (anywhere):%s
  --no-interactive   Disable interactive prompts (static rendering)
  --no-animation     Disable typewriter and animated effects
  --no-pagination    Disable press-to-continue between brief sections
  --plain            Shortcut for --no-interactive --no-animation --no-pagination
`, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset, colorBold, colorReset)
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
				starterFiles["main.go"] = `package main

import "fmt"

func main() {
	// ============================================================================
	// 🚀 LEVEL 1 KICKSTART: ONBOARDING INTRO
	// ============================================================================
	// Welcome! Level 1 is already solved for you so you can experience the
	// automated verification and testing workflow without friction.
	//
	// Step 1: UNCOMMENT the line below:
	// fmt.Print("$ ")
	//
	// Step 2: Save this file and run:
	//   mentor-client submit
	// ============================================================================
}
`
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

		if currentLevel == 1 {
			fmt.Printf("\n%s💡 LEVEL 1 KICKSTART:%s\n", colorYellow, colorReset)
			fmt.Println("  Level 1 is already solved for you in your starter code (main.go)!")
			fmt.Println("  Simply open main.go, uncomment the code block, and run 'mentor-client submit'.")
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
		ui.Println(ui.KeyVal("Workspace", workspaceDir))
	}
	ui.Println(ui.KeyVal("Course", ui.BoldStyle.Render(courseSlug)))
	ui.Println(ui.KeyVal("Level", ui.WarnStyle.Render(fmt.Sprintf("Level %d", currentLevel))))
	if coursePath != "" {
		ui.Println(ui.KeyVal("Source", coursePath))
	}

	if cfg != nil && coursePath != "" {
		levelID := levelIDForOrder(coursePath, currentLevel)
		if m, _ := milestoneForLevelID(coursePath, levelID); m != nil {
			ui.Println(ui.KeyVal("Milestone", m.MilestoneID))

			bf, _ := resolveBossFight(coursePath, *m)
			if bf != nil {
				state := "pending"
				if cfg.HasMilestone(m.MilestoneID) {
					state = "cleared"
				}
				ui.Println(ui.KeyVal("Boss Fight", fmt.Sprintf("%s [%s]", bf.Title, state)))
			}
		}
		ui.Println(ui.KeyVal("Milestones", fmt.Sprintf("%d cleared", len(cfg.CompletedMilestones))))
	}

	ui.Println()
	ui.Println(ui.MutedStyle.Render("Run `mentor-client brief` to inspect level objectives."))
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
	var language string

	_, cfg, err := findWorkspaceDir()
	if err == nil {
		coursePath = cfg.LocalPath
		courseSlug = cfg.CourseSlug
		currentLevel = cfg.CurrentLevel
		serverURL = cfg.ServerURL
		token = cfg.StudentToken
		language = cfg.Language
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

	ui.Println()
	ui.Println(ui.Banner(fmt.Sprintf("LEVEL %d BRIEF", currentLevel), strings.ToUpper(courseSlug)))

	if coursePath != "" {
		type levelInfo struct {
			ID         string `json:"id"`
			Order      int    `json:"order"`
			Title      string `json:"title"`
			Goal       string `json:"goal"`
			Difficulty string `json:"difficulty"`
			Narrative  struct {
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
		}

		var targetLevel *levelInfo
		var detectedLang string

		candidateBundlePaths := []string{
			filepath.Join(coursePath, "bundles", "curriculum.student.json"),
			filepath.Join(coursePath, "curriculum.student.json"),
			filepath.Join(coursePath, "bundles", "curriculum.full.json"),
			filepath.Join(coursePath, "curriculum.json"),
		}

		for _, bPath := range candidateBundlePaths {
			data, err := os.ReadFile(bPath)
			if err != nil {
				continue
			}
			var cur struct {
				Curriculum struct {
					TargetLanguage string `json:"target_language"`
				} `json:"curriculum"`
				TargetLanguage string      `json:"target_language"`
				Levels         []levelInfo `json:"levels"`
			}
			if err := json.Unmarshal(data, &cur); err == nil && len(cur.Levels) > 0 {
				if cur.Curriculum.TargetLanguage != "" {
					detectedLang = cur.Curriculum.TargetLanguage
				} else if cur.TargetLanguage != "" {
					detectedLang = cur.TargetLanguage
				}
				for i := range cur.Levels {
					if cur.Levels[i].Order == currentLevel {
						targetLevel = &cur.Levels[i]
						break
					}
				}
				if targetLevel != nil {
					break
				}
			}
		}

		// Fallback to individual level files if not found in bundle
		if targetLevel == nil {
			candidateLevelFiles := []string{
				filepath.Join(coursePath, "common", "levels", fmt.Sprintf("level-%02d.json", currentLevel)),
				filepath.Join(coursePath, "common", "levels", fmt.Sprintf("level-%d.json", currentLevel)),
			}
			for _, lPath := range candidateLevelFiles {
				data, err := os.ReadFile(lPath)
				if err != nil {
					continue
				}
				var lvl levelInfo
				if err := json.Unmarshal(data, &lvl); err == nil && (lvl.Title != "" || lvl.Goal != "") {
					targetLevel = &lvl
					break
				}
			}
		}

		if targetLevel != nil {
			lang := language
			if lang == "" {
				lang = detectedLang
			}
			if lang == "" {
				lang = "go"
			}

			ui.Println(ui.KeyVal("Title", ui.BoldStyle.Render(targetLevel.Title)+" "+ui.MutedStyle.Render("("+strings.ToUpper(targetLevel.Difficulty)+")")))
			ui.Println(ui.KeyVal("Goal", ui.Wrap(targetLevel.Goal)))
			ui.Println()

			// 1. Narrator — theatrical typewriter reveal
			if targetLevel.Narrative.Hook != "" {
				ui.Narrate(targetLevel.Narrative.Hook)
			}
			ui.Pause("")

			// 2. Special Level 1 Kickstart Notice
			if currentLevel == 1 {
				ui.Println(ui.Badge("⚡ LEVEL 1 KICKSTART"))
				ui.Println("  Level 1 is already solved for you in your starter code!")
				ui.Println("  Open main.go, simply UNCOMMENT the code block, and run 'mentor-client submit'.")
				ui.Println()
			}

			if targetLevel.Narrative.Motivation != "" {
				ui.Println(ui.Section("Why This Matters"))
				ui.Println(ui.Wrap(targetLevel.Narrative.Motivation))
				ui.Pause("")
			}
			if targetLevel.Narrative.ConcreteDemo != "" {
				ui.Println(ui.Section("Expected Demo"))
				ui.Println(ui.Wrap(targetLevel.Narrative.ConcreteDemo))
				ui.Pause("")
			}

			if len(targetLevel.LearningObjectives) > 0 {
				ui.Println(ui.Section("Learning Objectives"))
				for _, obj := range targetLevel.LearningObjectives {
					ui.Println(ui.Bullet(obj))
				}
				ui.Pause("")
			}

			if len(targetLevel.Constraints) > 0 {
				ui.Println(ui.Section("Public Constraints"))
				for _, c := range targetLevel.Constraints {
					ui.Println(ui.Bullet(c))
				}
				ui.Pause("")
			}

			if len(targetLevel.Deliverables) > 0 {
				ui.Println(ui.Section("Deliverables"))
				for _, d := range targetLevel.Deliverables {
					ui.Println(ui.Bullet(d))
				}
				ui.Pause("")
			}

			if len(targetLevel.Behaviors) > 0 {
				ui.Println(ui.Section("Required Behaviors"))
				for _, b := range targetLevel.Behaviors {
					ui.Println(ui.Bullet(ui.BoldStyle.Render("["+b.ID+"] ") + b.Description))
				}
				ui.Pause("")
			}

			// 3. Render Document Engine Artifacts (Theory, Language Cheat Sheet, Graded MCQs, Mini-Challenges)
			renderEducationalDocs(coursePath, targetLevel.ID, lang)

			// 4. Boss fight teaser at the final level of a milestone
			if m, final := isMilestoneFinalLevel(coursePath, targetLevel.ID); final {
				renderBossTeaser(coursePath, m)
			}

			return nil
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
				DemoCases []SpecCase `json:"demo_cases"`
			}
			if err := json.Unmarshal(data, &specData); err == nil {
				cases = specData.AgnosticTestSpec.Cases
				for i, dc := range specData.DemoCases {
					if dc.Name == "" {
						dc.Name = fmt.Sprintf("demo-%d", i+1)
					}
					cases = append(cases, dc)
				}
			}
		}

		if len(cases) == 0 {
			// Fallback: look in candidate bundle files
			candidateBundlePaths := []string{
				filepath.Join(coursePath, "curriculum.json"),
				filepath.Join(coursePath, "bundles", "curriculum.student.json"),
				filepath.Join(coursePath, "curriculum.student.json"),
				filepath.Join(coursePath, "bundles", "curriculum.full.json"),
			}
			for _, curPath := range candidateBundlePaths {
				data, err := os.ReadFile(curPath)
				if err != nil {
					continue
				}
				var cur struct {
					Levels []struct {
						Order            int `json:"order"`
						AgnosticTestSpec struct {
							Cases []SpecCase `json:"cases"`
						} `json:"agnostic_test_spec"`
						DemoCases []SpecCase `json:"demo_cases"`
					} `json:"levels"`
				}
				if err := json.Unmarshal(data, &cur); err == nil {
					for _, l := range cur.Levels {
						if l.Order == currentLevel {
							cases = l.AgnosticTestSpec.Cases
							for i, dc := range l.DemoCases {
								if dc.Name == "" {
									dc.Name = fmt.Sprintf("demo-%d", i+1)
								}
								cases = append(cases, dc)
							}
							break
						}
					}
					if len(cases) > 0 {
						break
					}
				}
			}
		}

		if len(cases) == 0 {
			// Fallback: look in individual common/levels/level-NN.json
			candidateLevelFiles := []string{
				filepath.Join(coursePath, "common", "levels", fmt.Sprintf("level-%02d.json", currentLevel)),
				filepath.Join(coursePath, "common", "levels", fmt.Sprintf("level-%d.json", currentLevel)),
			}
			for _, lPath := range candidateLevelFiles {
				data, err := os.ReadFile(lPath)
				if err != nil {
					continue
				}
				var lvl struct {
					AgnosticTestSpec struct {
						Cases []SpecCase `json:"cases"`
					} `json:"agnostic_test_spec"`
					DemoCases []SpecCase `json:"demo_cases"`
				}
				if err := json.Unmarshal(data, &lvl); err == nil {
					cases = lvl.AgnosticTestSpec.Cases
					for i, dc := range lvl.DemoCases {
						if dc.Name == "" {
							dc.Name = fmt.Sprintf("demo-%d", i+1)
						}
						cases = append(cases, dc)
					}
					if len(cases) > 0 {
						break
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
			switch strings.ToLower(strings.TrimSpace(tc.Matcher)) {
			case "trim", "":
				matched = (strings.TrimSpace(out) == strings.TrimSpace(tc.Expected))
			case "contains":
				matched = strings.Contains(out, tc.Expected)
			case "prefix":
				matched = strings.HasPrefix(out, tc.Expected)
			case "suffix":
				matched = strings.HasSuffix(out, tc.Expected)
			case "unordered_lines", "unordered":
				matched = matchUnorderedLines(out, tc.Expected)
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

		ui.Println(ui.SuccessStyle.Render(fmt.Sprintf("✔ [1/2] Predefined Verification: PASSED (%d/%d cases)", predefinedPassed, predefinedTotal)))
		if dynamicTotal > 0 {
			ui.Println(ui.SuccessStyle.Render(fmt.Sprintf("✔ [2/2] Dynamic Dual Oracle:     PASSED (%d/%d vectors)", dynamicPassed, dynamicTotal)))
		}
		ui.Println("\n" + ui.Success(fmt.Sprintf("🎉 LEVEL %d CERTIFIED VERIFIED!", currentLevel)))

		bossGated := false
		if cfg != nil {
			levelID := levelIDForOrder(coursePath, currentLevel)
			m, final := isMilestoneFinalLevel(coursePath, levelID)
			if final && !cfg.HasMilestone(m.MilestoneID) {
				if bf, _ := resolveBossFight(coursePath, *m); bf != nil && ui.Interactive() {
					bossGated = true
					if runBossFight(coursePath, *m, bf) {
						cfg.MarkMilestone(m.MilestoneID)
					} else {
						_ = config.SaveConfig(workspaceDir, cfg)
						ui.Println(ui.WarnStyle.Render("The boss fight was not cleared — re-run `mentor-client submit` to face it again before advancing."))
						return nil
					}
				}
			}
		}

		if cfg != nil {
			cfg.CurrentLevel++
			_ = config.SaveConfig(workspaceDir, cfg)
			if bossGated {
				ui.Println("\n" + ui.SuccessStyle.Render(fmt.Sprintf("Milestone cleared! Unlocked Level %d. Run `mentor-client brief` for the next challenge.", cfg.CurrentLevel)))
			} else {
				ui.Println(ui.MutedStyle.Render(fmt.Sprintf("Unlocked Level %d! Run `mentor-client brief` for the next challenge.", cfg.CurrentLevel)))
			}
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

// matchUnorderedLines compares two multiline strings as multisets of non-empty lines.
func matchUnorderedLines(actual, expected string) bool {
	actLines := extractNonEmptyLines(actual)
	expLines := extractNonEmptyLines(expected)
	if len(actLines) != len(expLines) {
		return false
	}
	counts := make(map[string]int)
	for _, l := range expLines {
		counts[l]++
	}
	for _, l := range actLines {
		counts[l]--
		if counts[l] < 0 {
			return false
		}
	}
	return true
}

func extractNonEmptyLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
