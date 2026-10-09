package api

// LevelManifest describes a public level in the curriculum manifest.
type LevelManifest struct {
	Order      int    `json:"order"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Goal       string `json:"goal"`
	Difficulty string `json:"difficulty"`
}

// CourseManifest represents the student-facing course outline.
type CourseManifest struct {
	CourseSlug  string          `json:"course_slug"`
	CourseTitle string          `json:"course_title"`
	Description string          `json:"description,omitempty"`
	Language    string          `json:"language"`
	Levels      []LevelManifest `json:"levels"`
}

// StarterCodeResponse returns the initial template files for a level.
type StarterCodeResponse struct {
	LevelOrder int               `json:"level_order"`
	Language   string            `json:"language"`
	Files      map[string]string `json:"files"`
}

// LevelBriefResponse contains the public pedagogical instructions and constraints for a level.
type LevelBriefResponse struct {
	LevelOrder          int      `json:"level_order"`
	Title               string   `json:"title"`
	Goal                string   `json:"goal"`
	Motivation          string   `json:"motivation,omitempty"`
	ConcreteDemo        string   `json:"concrete_demo,omitempty"`
	PublicConstraints   []string `json:"public_constraints,omitempty"`
	TotalHintsAvailable int      `json:"total_hints_available"`
}

// HintResponse returns a progressive hint for the level without spoilers.
type HintResponse struct {
	LevelOrder int    `json:"level_order"`
	Tier       int    `json:"tier"`
	HintType   string `json:"hint_type"`
	Content    string `json:"content"`
}

// SubmitRequest carries student files for grading.
type SubmitRequest struct {
	Language string            `json:"language"`
	Files    map[string]string `json:"files"`
}

// DualOracleReport details the two-phase verification results.
type DualOracleReport struct {
	PredefinedPassed bool `json:"predefined_passed"`
	PredefinedCount  int  `json:"predefined_count"`
	PredefinedTotal  int  `json:"predefined_total"`
	DynamicPassed    bool `json:"dynamic_passed"`
	DynamicCount     int  `json:"dynamic_count"`
	DynamicTotal     int  `json:"dynamic_total"`
}

// SubmitResponse carries authoritative grading results.
type SubmitResponse struct {
	Outcome     string            `json:"outcome"` // "passed", "test_failed", "compile_error", "timeout"
	Passed      bool              `json:"passed"`
	PassedCount int               `json:"passed_count"`
	TotalCount  int               `json:"total_count"`
	Oracle      *DualOracleReport `json:"oracle,omitempty"`
	Diagnostic  string            `json:"diagnostic,omitempty"`
}
