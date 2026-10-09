package main

import (
	"strings"
	"testing"
)

func TestEvaluateMCQsRequires100Percent(t *testing.T) {
	questions := []questionItem{
		{ID: "q-1", Question: "one", Options: []string{"a", "b", "c"}},
		{ID: "q-2", Question: "two", Options: []string{"a", "b", "c"}},
	}
	solutions := map[string]solutionQuestion{
		"q-1": {QuestionID: "q-1", AnswerIndex: 2},
		"q-2": {QuestionID: "q-2", AnswerIndex: 0},
	}

	tests := []struct {
		name    string
		answers map[string]int
		correct int
		total   int
		passed  bool
	}{
		{"all correct passes", map[string]int{"q-1": 2, "q-2": 0}, 2, 2, true},
		{"one wrong fails", map[string]int{"q-1": 1, "q-2": 0}, 1, 2, false},
		{"all wrong fails", map[string]int{"q-1": 0, "q-2": 2}, 0, 2, false},
		{"missing answer fails", map[string]int{"q-1": 2}, 1, 2, false},
		{"unknown question ignored", map[string]int{"q-1": 2, "q-9": 0}, 1, 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			correct, total, passed := evaluateMCQs(questions, solutions, tc.answers)
			if correct != tc.correct || total != tc.total || passed != tc.passed {
				t.Errorf("got (%d/%d, passed=%v), want (%d/%d, passed=%v)",
					correct, total, passed, tc.correct, tc.total, tc.passed)
			}
		})
	}
}

func TestEvaluateBossMCQsRequires100Percent(t *testing.T) {
	bf, _ := resolveBossFight(mockCourse, milestoneForLevelTest(t))
	if bf == nil || len(bf.ConceptMCQs) == 0 {
		t.Skip("no boss fight with MCQs available")
	}

	correct := map[string]int{}
	for _, q := range bf.ConceptMCQs {
		correct[q.ID] = q.AnswerIndex
	}

	// Flawless run.
	c, total, passed := evaluateBossMCQs(bf.ConceptMCQs, correct)
	if !passed || c != total {
		t.Fatalf("flawless boss run should pass, got %d/%d passed=%v", c, total, passed)
	}

	// One wrong answer must fail.
	wrong := map[string]int{}
	for i, q := range bf.ConceptMCQs {
		if i == 0 {
			wrong[q.ID] = (q.AnswerIndex + 1) % len(q.Options)
		} else {
			wrong[q.ID] = q.AnswerIndex
		}
	}
	_, total, passed = evaluateBossMCQs(bf.ConceptMCQs, wrong)
	if passed || total == 0 {
		t.Fatalf("boss run with one wrong answer must fail, got passed=%v total=%d", passed, total)
	}
}

func milestoneForLevelTest(t *testing.T) milestoneEntry {
	t.Helper()
	m, _ := milestoneForLevelID(mockCourse, "display-shell-prompt-level")
	if m == nil {
		t.Fatal("no milestone for level 1")
	}
	return *m
}

func TestBossFightTitleRelevant(t *testing.T) {
	bf, _ := resolveBossFight(mockCourse, milestoneForLevelTest(t))
	if bf == nil {
		t.Fatal("expected boss fight")
	}
	if !strings.Contains(bf.Title, "REPL") && !strings.Contains(bf.Title, "Shell") {
		t.Errorf("unexpected boss title %q", bf.Title)
	}
}
