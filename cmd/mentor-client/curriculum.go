package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// bundleLevel is the minimal level identity used to map numeric progression
// (CurrentLevel) to the curriculum's string level ids.
type bundleLevel struct {
	Order int    `json:"order"`
	ID    string `json:"id"`
}

// milestoneManifest mirrors the parts of a milestone manifest the client needs.
type milestoneManifest struct {
	MilestoneID     string `json:"milestone_id"`
	Order           int    `json:"order"`
	ServerArtifacts struct {
		BossfightContract string `json:"bossfight_contract"`
	} `json:"server_artifacts"`
	LearningPath []struct {
		LevelID string `json:"level_id"`
		Unlocks struct {
			Theory     []string `json:"theory"`
			Questions  []string `json:"questions"`
			Challenges []string `json:"challenges"`
		} `json:"unlocks"`
	} `json:"learning_path"`
}

// milestoneEntry pairs a parsed manifest with its on-disk location.
type milestoneEntry struct {
	Dir  string
	Base string
	milestoneManifest
}

// candidateBundles lists curriculum bundle files (without extension),
// student-facing first.
func candidateBundles(coursePath string) []string {
	return []string{
		filepath.Join(coursePath, "bundles", "curriculum.student"),
		filepath.Join(coursePath, "curriculum.student"),
		filepath.Join(coursePath, "bundles", "curriculum.full"),
		filepath.Join(coursePath, "curriculum"),
	}
}

// loadBundleLevels reads the first bundle that yields a non-empty level list.
func loadBundleLevels(coursePath string) []bundleLevel {
	for _, base := range candidateBundles(coursePath) {
		var cur struct {
			Levels []bundleLevel `json:"levels"`
		}
		if _, ok := loadCourseFile(extCandidates(base), &cur); ok && len(cur.Levels) > 0 {
			return cur.Levels
		}
	}
	return nil
}

// levelIDForOrder maps a numeric level order to its string id, falling back to
// the individual level files when no bundle is present.
func levelIDForOrder(coursePath string, order int) string {
	for _, l := range loadBundleLevels(coursePath) {
		if l.Order == order {
			return l.ID
		}
	}

	for _, name := range []string{
		fmt.Sprintf("level-%02d", order),
		fmt.Sprintf("level-%d", order),
	} {
		var lvl struct {
			ID string `json:"id"`
		}
		if _, ok := loadCourseFile(extCandidates(filepath.Join(coursePath, "common", "levels", name)), &lvl); ok && lvl.ID != "" {
			return lvl.ID
		}
	}
	return ""
}

// listMilestones parses every milestone manifest under common/milestones.
func listMilestones(coursePath string) []milestoneEntry {
	root := filepath.Join(coursePath, "common", "milestones")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var out []milestoneEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		var mf milestoneManifest
		if _, ok := loadCourseFile(extCandidates(filepath.Join(dir, "manifest")), &mf); !ok {
			continue
		}
		out = append(out, milestoneEntry{Dir: dir, Base: e.Name(), milestoneManifest: mf})
	}
	return out
}

// milestoneForLevelID finds the milestone containing levelID and its position
// within that milestone's learning path.
func milestoneForLevelID(coursePath, levelID string) (*milestoneEntry, int) {
	if levelID == "" {
		return nil, -1
	}
	for _, m := range listMilestones(coursePath) {
		for i, lp := range m.LearningPath {
			if lp.LevelID == levelID {
				mc := m
				return &mc, i
			}
		}
	}
	return nil, -1
}

// isMilestoneFinalLevel reports whether levelID is the last step of a milestone.
func isMilestoneFinalLevel(coursePath, levelID string) (*milestoneEntry, bool) {
	m, idx := milestoneForLevelID(coursePath, levelID)
	if m == nil || idx < 0 {
		return nil, false
	}
	return m, idx == len(m.LearningPath)-1
}

// bossFight is the parsed boss fight contract.
type bossFight struct {
	Title                 string   `json:"title"`
	PublicPreview         string   `json:"public_preview"`
	IntegratedBehaviorIDs []string `json:"integrated_behavior_ids"`
	Contract              struct {
		Scenario string `json:"scenario"`
		Steps    []struct {
			Action          string `json:"action"`
			ExpectedOutcome string `json:"expected_outcome"`
		} `json:"steps"`
	} `json:"contract"`
	ConceptMCQs []struct {
		ID          string   `json:"id"`
		Question    string   `json:"question"`
		Options     []string `json:"options"`
		AnswerIndex int      `json:"answer_index"`
		Explanation string   `json:"explanation"`
	} `json:"concept_mcqs"`
}

// resolveBossFight locates a milestone's boss fight across the layered
// curriculum layout. It returns the fight and the source path that provided it.
func resolveBossFight(coursePath string, m milestoneEntry) (*bossFight, string) {
	candidates := []string{}
	if ref := m.ServerArtifacts.BossfightContract; ref != "" {
		candidates = extCandidates(
			filepath.Join(m.Dir, ref),
			filepath.Join(coursePath, "common", "server", "milestones", ref),
			filepath.Join(coursePath, "server", "milestones", ref),
		)
	}
	for _, p := range candidates {
		var bf bossFight
		if _, ok := loadCourseFile([]string{p}, &bf); ok && (bf.Title != "" || len(bf.ConceptMCQs) > 0) {
			return &bf, p
		}
	}

	// Fallback: the instructor bundle carries complete boss fights keyed by id
	// (the student bundle strips concept answers and contract steps).
	for _, base := range []string{
		filepath.Join(coursePath, "bundles", "curriculum.full"),
		filepath.Join(coursePath, "curriculum"),
		filepath.Join(coursePath, "bundles", "curriculum.student"),
	} {
		var cur struct {
			Milestones []struct {
				ID        string    `json:"id"`
				BossFight bossFight `json:"boss_fight"`
			} `json:"milestones"`
		}
		if _, ok := loadCourseFile(extCandidates(base), &cur); ok {
			for _, ms := range cur.Milestones {
				if ms.ID == m.MilestoneID && (ms.BossFight.Title != "" || len(ms.BossFight.ConceptMCQs) > 0) {
					return &ms.BossFight, base
				}
			}
		}
	}
	return nil, ""
}

// solutionQuestion is a graded answer key from document-solutions.json.
type solutionQuestion struct {
	QuestionID  string `json:"question_id"`
	LevelID     string `json:"level_id"`
	AnswerIndex int    `json:"answer_index"`
	Explanation string `json:"explanation"`
}

// loadSolutions reads the server-side answer key for a milestone, if present.
func loadSolutions(coursePath, milestoneBase string) map[string]solutionQuestion {
	out := map[string]solutionQuestion{}
	paths := []string{
		filepath.Join(coursePath, "server", "milestones", milestoneBase, "document-solutions"),
		filepath.Join(coursePath, "common", "server", "milestones", milestoneBase, "document-solutions"),
	}
	var sol struct {
		Questions []solutionQuestion `json:"questions"`
	}
	if _, ok := loadCourseFile(extCandidates(paths...), &sol); ok {
		for _, q := range sol.Questions {
			out[q.QuestionID] = q
		}
	}
	return out
}
