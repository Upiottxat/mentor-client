package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIClient(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/curriculum/simple-shell/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(CourseManifest{
			CourseSlug:  "simple-shell",
			CourseTitle: "Simple Shell",
			Language:    "go",
			Levels: []LevelManifest{
				{Order: 1, ID: "repl-loop", Title: "REPL Loop", Difficulty: "easy"},
			},
		})
	})

	mux.HandleFunc("/v1/level/simple-shell/1/starter", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(StarterCodeResponse{
			LevelOrder: 1,
			Language:   "go",
			Files: map[string]string{
				"main.go": "package main\n",
			},
		})
	})

	mux.HandleFunc("/v1/level/simple-shell/1/brief", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(LevelBriefResponse{
			LevelOrder: 1,
			Title:      "REPL Loop",
			Goal:       "Create interactive shell loop",
		})
	})

	mux.HandleFunc("/v1/level/simple-shell/1/submit", func(w http.ResponseWriter, r *http.Request) {
		var req SubmitRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(SubmitResponse{
			Outcome:     "passed",
			Passed:      true,
			PassedCount: 5,
			TotalCount:  5,
			Oracle: &DualOracleReport{
				PredefinedPassed: true,
				PredefinedCount:  3,
				PredefinedTotal:  3,
				DynamicPassed:    true,
				DynamicCount:     2,
				DynamicTotal:     2,
			},
			Diagnostic: "All 5/5 test cases passed!",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient(server.URL, "test_token")
	ctx := context.Background()

	manifest, err := client.GetManifest(ctx, "simple-shell")
	if err != nil {
		t.Fatalf("GetManifest failed: %v", err)
	}
	if manifest.CourseSlug != "simple-shell" || len(manifest.Levels) != 1 {
		t.Errorf("unexpected manifest: %+v", manifest)
	}

	starter, err := client.GetStarter(ctx, "simple-shell", 1)
	if err != nil {
		t.Fatalf("GetStarter failed: %v", err)
	}
	if starter.Files["main.go"] != "package main\n" {
		t.Errorf("unexpected starter files: %+v", starter.Files)
	}

	brief, err := client.GetBrief(ctx, "simple-shell", 1)
	if err != nil {
		t.Fatalf("GetBrief failed: %v", err)
	}
	if brief.Title != "REPL Loop" {
		t.Errorf("unexpected brief: %+v", brief)
	}

	submitRes, err := client.Submit(ctx, "simple-shell", 1, "go", map[string]string{"main.go": "package main\n"})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if !submitRes.Passed || submitRes.Oracle == nil || !submitRes.Oracle.DynamicPassed {
		t.Errorf("unexpected submit result: %+v", submitRes)
	}
}
