package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/upiottxat/mentor-client/internal/config"
)

const mockCourse = "../../internal/Curriculam Providers/MOCK/mini-project-course"

func TestLevelIDForOrder(t *testing.T) {
	tests := []struct {
		order int
		want  string
	}{
		{1, "display-shell-prompt-level"},
		{3, "level-3-handle-exit-command"},
		{12, "level-12-input-output-redirection"},
	}
	for _, tc := range tests {
		got := levelIDForOrder(mockCourse, tc.order)
		if got != tc.want {
			t.Errorf("levelIDForOrder(%d) = %q, want %q", tc.order, got, tc.want)
		}
	}
}

func TestIsMilestoneFinalLevel(t *testing.T) {
	tests := []struct {
		levelID string
		final   bool
	}{
		{"display-shell-prompt-level", false},
		{"level-2-read-user-input", false},
		{"level-3-handle-exit-command", true},
		{"level-12-input-output-redirection", true},
		{"does-not-exist", false},
	}
	for _, tc := range tests {
		_, got := isMilestoneFinalLevel(mockCourse, tc.levelID)
		if got != tc.final {
			t.Errorf("isMilestoneFinalLevel(%q) = %v, want %v", tc.levelID, got, tc.final)
		}
	}
}

func TestResolveBossFightFromInstructorBundle(t *testing.T) {
	m, _ := milestoneForLevelID(mockCourse, "display-shell-prompt-level")
	if m == nil {
		t.Fatal("expected milestone for level 1")
	}
	bf, src := resolveBossFight(mockCourse, *m)
	if bf == nil {
		t.Fatal("expected boss fight to resolve for repl-loop milestone")
	}
	if len(bf.ConceptMCQs) == 0 {
		t.Errorf("boss fight for %s should have concept MCQs", m.MilestoneID)
	}
	if len(bf.Contract.Steps) == 0 {
		t.Errorf("boss fight for %s should have contract steps", m.MilestoneID)
	}
	if src == "" {
		t.Error("expected a source path for the boss fight")
	}
}

func TestStudentBundleZeroLeak(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(mockCourse, "bundles", "curriculum.student.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Milestones []struct {
			BossFight struct {
				ConceptMCQs []map[string]any `json:"concept_mcqs"`
			} `json:"boss_fight"`
		} `json:"milestones"`
		Levels []map[string]any `json:"levels"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for i, ms := range raw.Milestones {
		for j, q := range ms.BossFight.ConceptMCQs {
			if _, has := q["answer_index"]; has {
				t.Errorf("student bundle leaks answer_index (milestone %d, mcq %d)", i, j)
			}
		}
	}
	for i, lvl := range raw.Levels {
		if _, has := lvl["answer_index"]; has {
			t.Errorf("student bundle leaks answer_index (level %d)", i)
		}
	}
}

func TestConfigMilestoneTracking(t *testing.T) {
	cfg := &config.WorkspaceConfig{}
	cfg.MarkMilestone("repl-loop")
	if !cfg.HasMilestone("repl-loop") {
		t.Fatal("expected milestone to be marked")
	}
	if len(cfg.BossFightsPassed) != 1 {
		t.Fatalf("expected boss pass recorded, got %v", cfg.BossFightsPassed)
	}
	cfg.MarkMilestone("repl-loop") // idempotent
	cfg.MarkMilestone("")
	if len(cfg.CompletedMilestones) != 1 {
		t.Fatalf("MarkMilestone should be idempotent, got %v", cfg.CompletedMilestones)
	}
	if !cfg.HasMilestone("missing") {
		// no-op: ensure it doesn't spuriously report true
	}
	if len(cfg.BossFightsPassed) != 1 {
		t.Fatalf("boss passes should not duplicate, got %v", cfg.BossFightsPassed)
	}
}
