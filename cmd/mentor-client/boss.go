package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/upiottxat/mentor-client/internal/ui"
)

// bossBanner renders the dramatic boss-fight title panel.
func bossBanner(title, preview string) string {
	head := ui.Gradient("⚔️  BOSS FIGHT", "#F87171", "#FBBF24")
	body := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FCA5A5")).Render(title)
	if preview != "" {
		body += "\n" + ui.MutedStyle.Render(preview)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#F87171")).
		Padding(1, 3).
		MarginTop(1).
		MarginBottom(1).
		Render(head + "\n\n" + body)
}

// renderBossTeaser shows an upcoming boss fight at the end of a milestone.
func renderBossTeaser(coursePath string, m *milestoneEntry) {
	if m == nil {
		return
	}
	bf, _ := resolveBossFight(coursePath, *m)
	if bf == nil {
		return
	}
	ui.Println("\n" + ui.DangerBadge.Render(" ⚠ BOSS FIGHT AHEAD "))
	ui.Println(ui.BoldStyle.Render("  " + bf.Title))
	if bf.PublicPreview != "" {
		ui.Println("  " + ui.MutedStyle.Render(ui.Wrap(bf.PublicPreview)))
	}
	ui.Println(ui.MutedStyle.Render("  Complete this final level and submit to face the boss."))
}

// runBossFight runs the milestone boss fight, requiring a perfect result. It
// returns true only when every scenario step is confirmed and every concept MCQ
// is answered correctly.
func runBossFight(coursePath string, m milestoneEntry, bf *bossFight) bool {
	if bf == nil {
		return false
	}

	if !ui.Interactive() {
		renderBossTeaser(coursePath, &m)
		ui.Println(ui.WarnStyle.Render("  The boss fight needs an interactive terminal — run `mentor-client submit` in a TTY."))
		return false
	}

	ui.Println(bossBanner(bf.Title, bf.PublicPreview))

	for {
		if attemptBoss(bf) {
			ui.Println("\n" + ui.Success("BOSS DEFEATED — 100% CLEARED"))
			return true
		}

		ui.Println("\n" + ui.Danger("BOSS FIGHT FAILED — a perfect result is required to proceed."))
		retry, err := ui.Confirm("Face the boss again?", "Retry", "Retreat")
		if err != nil || !retry {
			return false
		}
	}
}

// evaluateBossMCQs grades the concept-trial answers. A boss trial only passes
// when every MCQ is answered and every answer is correct.
func evaluateBossMCQs(mcqs []struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	AnswerIndex int      `json:"answer_index"`
	Explanation string   `json:"explanation"`
}, answers map[string]int) (correct, total int, passed bool) {
	for _, q := range mcqs {
		total++
		if a, ok := answers[q.ID]; ok && a == q.AnswerIndex {
			correct++
		}
	}
	return correct, total, total > 0 && correct == total
}

func attemptBoss(bf *bossFight) bool {
	// 1. Scenario walkthrough.
	if bf.Contract.Scenario != "" {
		ui.Narrate(bf.Contract.Scenario)
	}

	for i, step := range bf.Contract.Steps {
		ui.Println(ui.BoldStyle.Render(fmt.Sprintf("Step %d/%d", i+1, len(bf.Contract.Steps))))
		ui.Println("  " + ui.InfoStyle.Render("Action:   ") + ui.Wrap(step.Action))
		ui.Println("  " + ui.InfoStyle.Render("Expected: ") + ui.Wrap(step.ExpectedOutcome))

		ok, err := ui.Confirm("Did your implementation behave exactly as expected?", "Yes", "No")
		if err != nil || !ok {
			ui.Println(ui.MutedStyle.Render("  Scenario step not satisfied."))
			return false
		}
	}

	// 2. Concept MCQ gauntlet — must be flawless.
	if len(bf.ConceptMCQs) > 0 {
		ui.Println("\n" + ui.Section("🎯 Boss Concept Trials"))
	}

	answers := map[string]int{}
	for i, q := range bf.ConceptMCQs {
		ui.Println(ui.BoldStyle.Render(fmt.Sprintf("[%d/%d] %s", i+1, len(bf.ConceptMCQs), q.Question)))

		choice, err := ui.Select("Choose your answer", q.Options)
		if err != nil {
			return false
		}
		answers[q.ID] = choice

		if choice == q.AnswerIndex {
			ui.Println(ui.SuccessStyle.Render("  ✅ Correct!"))
		} else {
			ui.Println(ui.DangerStyle.Render(fmt.Sprintf("  ❌ Incorrect — correct answer: %d) %s", q.AnswerIndex+1, q.Options[q.AnswerIndex])))
		}
		if q.Explanation != "" {
			ui.Println(ui.HintStyle.Render("💡 " + q.Explanation))
		}
	}

	if len(bf.ConceptMCQs) == 0 {
		return true
	}

	correct, total, passed := evaluateBossMCQs(bf.ConceptMCQs, answers)
	if !passed {
		ui.Println(ui.DangerStyle.Render(fmt.Sprintf("Concept trials: %d/%d — a perfect score is required.", correct, total)))
	}
	return passed
}
