package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCourse(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type yamlFixtureCurriculum struct {
	Curriculum struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		TargetLanguage string `json:"target_language"`
	} `json:"curriculum"`
	Milestones []struct {
		ID        string `json:"id"`
		Order     int    `json:"order"`
		BossFight struct {
			Title        string `json:"title"`
			PublicReview string `json:"public_preview"`
			ConceptMCQs  []struct {
				ID          string   `json:"id"`
				Question    string   `json:"question"`
				Options     []string `json:"options"`
				AnswerIndex int      `json:"answer_index"`
			} `json:"concept_mcqs"`
		} `json:"boss_fight"`
	} `json:"milestones"`
}

const fixtureYAML = `
curriculum:
  id: demo-course
  name: Demo Course
  target_language: cpp
milestones:
  - id: ms-1
    order: 1
    boss_fight:
      title: The Terminal Tyrant
      public_preview: fight preview
      concept_mcqs:
        - id: bq-1
          question: Which signal catches EOF?
          options: ["SIGINT", "EOF", "SIGTERM"]
          answer_index: 1
`

func TestDecodeCourseDataYAML(t *testing.T) {
	var cur yamlFixtureCurriculum
	if err := decodeCourseData([]byte(fixtureYAML), &cur); err != nil {
		t.Fatalf("decode yaml: %v", err)
	}
	if cur.Curriculum.ID != "demo-course" {
		t.Errorf("id: got %q", cur.Curriculum.ID)
	}
	if cur.Curriculum.Name != "Demo Course" {
		t.Errorf("name: got %q", cur.Curriculum.Name)
	}
	if cur.Curriculum.TargetLanguage != "cpp" {
		t.Errorf("target_language: got %q", cur.Curriculum.TargetLanguage)
	}
	if cur.Milestones[0].ID != "ms-1" || cur.Milestones[0].Order != 1 {
		t.Errorf("milestone[0]: got id=%q order=%d", cur.Milestones[0].ID, cur.Milestones[0].Order)
	}
	bf := cur.Milestones[0].BossFight
	if bf.Title != "The Terminal Tyrant" {
		t.Errorf("boss_fight.title: got %q", bf.Title)
	}
	if len(bf.ConceptMCQs) != 1 || bf.ConceptMCQs[0].AnswerIndex != 1 || bf.ConceptMCQs[0].Options[2] != "SIGTERM" {
		t.Errorf("boss_fight concept_mcqs not decoded: %+v", bf.ConceptMCQs)
	}
}

func TestDecodeCourseDataJSON(t *testing.T) {
	var cur yamlFixtureCurriculum
	if err := decodeCourseData([]byte(`{"curriculum":{"id":"demo-course","name":"JSON Course","target_language":"go"},"milestones":[{"id":"ms-1","order":1}]}`), &cur); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if cur.Curriculum.ID != "demo-course" || cur.Curriculum.Name != "JSON Course" {
		t.Errorf("bad json decode: %+v", cur.Curriculum)
	}
}

func TestClassifyCourseInput(t *testing.T) {
	if abs, local, err := classifyCourseInput("advanced-bash-shell-cpp"); err != nil || local || abs == "" {
		t.Errorf("slug: got local=%v err=%v", local, err)
	}

	if _, local, err := classifyCourseInput("./does/not/exist"); err == nil || local {
		t.Errorf("missing path: want error & !local, got local=%v err=%v", local, err)
	} else if !strings.Contains(err.Error(), "course directory not found") {
		t.Errorf("missing path: unexpected error %q", err)
	}

	if _, local, err := classifyCourseInput("/no/such/course"); err == nil || local {
		t.Errorf("absolute missing path: want error, got local=%v err=%v", local, err)
	}

	course := writeFixtureCourse(t, nil)
	if abs, local, err := classifyCourseInput(course); err != nil || !local || filepath.Clean(abs) != filepath.Clean(course) {
		t.Errorf("existing dir: got local=%v abs=%q err=%v", local, abs, err)
	}
}

func TestLevelStarterFilesYAML(t *testing.T) {
	root := writeFixtureCourse(t, map[string]string{
		"common/levels/level-01.yaml": `id: level-01
title: Level One
starter_code:
  files:
    main.py: |
      print("hello local")
`,
	})
	files := levelStarterFiles(root, "python")
	got, ok := files["main.py"]
	if !ok || !strings.Contains(got, "hello local") {
		t.Errorf("starter files: got %#v", files)
	}

	// Reference solution fallback when starter_code absent.
	root2 := writeFixtureCourse(t, map[string]string{
		"common/levels/level-01.yaml": `id: level-01
reference_solution:
  files:
    shell.cpp: |
      int main() {}
`,
	})
	files2 := levelStarterFiles(root2, "cpp")
	if _, ok := files2["shell.cpp"]; !ok {
		t.Errorf("reference fallback: got %#v", files2)
	}
}

func TestResolveBossFightYAML(t *testing.T) {
	root := writeFixtureCourse(t, map[string]string{
		"curriculum.yaml": fixtureYAML,
		"common/levels/level-01.yaml": `id: level-01
title: One
`,
	})
	m := milestoneEntry{milestoneManifest: milestoneManifest{MilestoneID: "ms-1"}}
	bf, src := resolveBossFight(root, m)
	if bf == nil {
		t.Fatalf("resolveBossFight from curriculum.yaml returned nil")
	}
	if bf.Title != "The Terminal Tyrant" {
		t.Errorf("boss title: got %q", bf.Title)
	}
	if src == "" {
		t.Errorf("expected a source path")
	}
}

func TestLevelIDForOrderYAML(t *testing.T) {
	root := writeFixtureCourse(t, map[string]string{
		"common/levels/level-02.yaml": `id: interactive-repl-loop
title: Repl Loop
`,
	})
	if got := levelIDForOrder(root, 2); got != "interactive-repl-loop" {
		t.Errorf("levelIDForOrder: got %q", got)
	}
}

func TestListMilestonesYAML(t *testing.T) {
	root := writeFixtureCourse(t, map[string]string{
		"common/milestones/milestone-01/manifest.yaml": `milestone_id: ms-foundation
order: 1
server_artifacts:
  bossfight_contract: milestone-01.bossfight.yaml
learning_path:
  - level_id: level-01
    unlocks:
      theory: [theo-1]
      questions: [q-1]
      challenges: [c-1]
`,
	})
	milestones := listMilestones(root)
	if len(milestones) != 1 {
		t.Fatalf("listMilestones: got %d entries", len(milestones))
	}
	m := milestones[0]
	if m.MilestoneID != "ms-foundation" || m.Base != "milestone-01" || len(m.LearningPath) != 1 || m.LearningPath[0].Unlocks.Theory[0] != "theo-1" {
		t.Errorf("manifest decode wrong: %+v", m)
	}
}

func writeFixtureCourse(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "course")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(files) > 0 {
		writeCourse(t, root, files)
	}
	return root
}

func TestDecodeCourseDataTabIndentedYAML(t *testing.T) {
	const doc = "id: lvl\ncases:\n\t- name: a\n\t  expected: x\n"
	var out struct {
		ID    string `json:"id"`
		Cases []struct {
			Name     string `json:"name"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	if err := decodeCourseData([]byte(doc), &out); err != nil {
		t.Fatalf("tab-indented yaml should decode after repair: %v", err)
	}
	if out.ID != "lvl" || len(out.Cases) != 1 || out.Cases[0].Name != "a" || out.Cases[0].Expected != "x" {
		t.Errorf("decoded wrong: %+v", out)
	}
}

func TestRepairYAMLIndentationKeepsContentTabs(t *testing.T) {
	in := []byte("v: \"a\tb\"\nnext: 1\n")
	if got := repairYAMLIndentation(in); string(got) != string(in) {
		t.Errorf("content tabs must be preserved: got %q", got)
	}
}
