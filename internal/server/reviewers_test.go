package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/i18n"
)

// Fake reviewers for the health and auto-pick tests. Each is a /bin/sh script
// that receives the prompt as $1.
var fakeReviewers = map[string]string{
	// Answers the smoke test and prints the fake report for real runs.
	"fake-smart": `case "$1" in *XPLAN-OK*) printf 'XPLAN-OK' ;; *) printf '%s' "$XPLAN_FAKE" ;; esac`,
	// Always fails the way a logged-out CLI does, colors included.
	"fake-auth": `printf '\033[91mError:\033[0m Authentication required. Please log in first.\n' >&2; exit 1`,
	// Passes the smoke test, then hits a login wall on the real run.
	"fake-flaky": `case "$1" in *XPLAN-OK*) printf 'XPLAN-OK' ;; *) echo 'Error: subscription expired' >&2; exit 1 ;; esac`,
}

func (e *testEnv) addFakeReviewers() {
	e.t.Helper()
	_, err := e.app.UpdateConfig(func(cfg *config.Config) error {
		for name, script := range fakeReviewers {
			cfg.Reviewers[name] = config.ReviewerDef{
				Description: "test double",
				Command:     []string{"/bin/sh", "-c", script, "sh", "{prompt}"},
			}
		}
		cfg.Reviewers["fake-missing"] = config.ReviewerDef{
			Description: "test double that is not installed",
			Command:     []string{"xplan-no-such-agent-binary", "{prompt}"},
		}
		return nil
	})
	if err != nil {
		e.t.Fatalf("configure fake reviewers: %v", err)
	}
}

func TestReviewerHealthCheck(t *testing.T) {
	env := newTestEnv(t)
	env.addFakeReviewers()

	cases := map[string]string{
		"fake-smart":   "ok",
		"fake-auth":    "auth",
		"fake-missing": "missing",
	}
	for name, want := range cases {
		result := env.expect("POST", "/api/reviewers/"+name+"/test", nil, http.StatusOK)
		health, _ := result["health"].(map[string]any)
		if health["status"] != want {
			t.Fatalf("%s: health = %v, want status %q", name, result, want)
		}
		if label, _ := result["health_label"].(string); label == "" {
			t.Fatalf("%s: missing health_label in %v", name, result)
		}
		if detail, _ := health["detail"].(string); strings.Contains(detail, "\x1b") {
			t.Fatalf("%s: detail keeps terminal color codes: %q", name, detail)
		}
	}
	env.expect("POST", "/api/reviewers/ghost/test", nil, http.StatusBadRequest)

	meta := env.expect("GET", "/api/meta", nil, http.StatusOK)
	reviewers, _ := meta["reviewers"].([]any)
	found := false
	for _, raw := range reviewers {
		info, _ := raw.(map[string]any)
		if info["name"] != "fake-smart" {
			continue
		}
		found = true
		health, _ := info["health"].(map[string]any)
		if health["status"] != "ok" || info["health_label"] == "" {
			t.Fatalf("meta reviewer fake-smart = %v", info)
		}
	}
	if !found {
		t.Fatalf("meta reviewers do not list fake-smart: %v", reviewers)
	}
	for _, key := range []string{"reviewer_order", "auto_choice", "language"} {
		if _, ok := meta[key]; !ok {
			t.Fatalf("meta missing %q", key)
		}
	}
}

func TestAutoReviewerFallsBack(t *testing.T) {
	env := newTestEnv(t)
	env.addFakeReviewers()
	slug := env.addProject()
	base := "/api/projects/" + slug

	env.expect("PATCH", "/api/config", map[string]any{"reviewer_order": []string{"fake-missing", "ghost"}}, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{
		"reviewer_order": []string{"fake-missing", "fake-auth", "fake-flaky", "fake-smart"},
	}, http.StatusOK)
	env.expect("PATCH", base, map[string]any{"reviewer": "auto"}, http.StatusOK)

	started := env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusAccepted)
	if started["reviewer_requested"] != "auto" {
		t.Fatalf("queued job should remember auto: %v", started)
	}
	jobID, _ := started["id"].(string)
	job := waitForJob(t, env, base, jobID)

	if job["status"] != "done" || job["verdict"] != "attention" {
		t.Fatalf("job = status %v verdict %v error %v", job["status"], job["verdict"], job["error"])
	}
	if job["reviewer"] != "fake-smart" {
		t.Fatalf("auto picked %v, want fake-smart (attempts %v)", job["reviewer"], job["attempts"])
	}
	attempts, _ := job["attempts"].([]any)
	if len(attempts) != 3 {
		t.Fatalf("attempts = %v, want missing, auth and the failed real run", attempts)
	}

	meta := env.expect("GET", "/api/meta", nil, http.StatusOK)
	choice, _ := meta["auto_choice"].(map[string]any)
	if choice["name"] != "fake-smart" || choice["verified"] != true {
		t.Fatalf("auto_choice = %v, want verified fake-smart", choice)
	}
}

func TestAutoReviewerWithNothingWorking(t *testing.T) {
	env := newTestEnv(t)
	env.addFakeReviewers()
	slug := env.addProject()
	base := "/api/projects/" + slug

	// Only broken candidates, and every other preset pointed at a missing
	// binary so the test never reaches a real agent on this machine.
	_, err := env.app.UpdateConfig(func(cfg *config.Config) error {
		for name, def := range cfg.Reviewers {
			if name != "fake-auth" {
				def.Command = []string{"xplan-no-such-agent-binary"}
				cfg.Reviewers[name] = def
			}
		}
		cfg.ReviewerOrder = []string{"fake-auth"}
		cfg.DefaultReviewer = config.AutoReviewer
		return nil
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}

	started := env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusAccepted)
	jobID, _ := started["id"].(string)
	job := waitForJob(t, env, base, jobID)
	if job["status"] != "failed" {
		t.Fatalf("job should fail without a working reviewer: %v", job)
	}
	if message, _ := job["error"].(string); message == "" {
		t.Fatalf("failed job should explain why: %v", job)
	}
}

func TestLanguageSwitch(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { i18n.SetLanguage("en") })

	env.expect("PATCH", "/api/config", map[string]any{"language": "fr"}, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{"language": "zh"}, http.StatusOK)

	meta := env.expect("GET", "/api/meta", nil, http.StatusOK)
	if meta["language"] != "zh" {
		t.Fatalf("meta language = %v", meta["language"])
	}
	kinds, _ := meta["review_kinds"].([]any)
	first, _ := kinds[0].(map[string]any)
	if first["title"] != "偏离检查" {
		t.Fatalf("review kind title should be Chinese, got %v", first["title"])
	}

	slug := env.addProject()
	reply := env.expect("POST", "/api/projects/"+slug+"/console", map[string]string{"input": "/note 你好"}, http.StatusOK)
	if reply["output"] != "已记录。" {
		t.Fatalf("console reply = %v", reply)
	}

	env.expect("PATCH", "/api/config", map[string]any{"language": "en"}, http.StatusOK)
	meta = env.expect("GET", "/api/meta", nil, http.StatusOK)
	kinds, _ = meta["review_kinds"].([]any)
	first, _ = kinds[0].(map[string]any)
	if first["title"] != "Drift check" {
		t.Fatalf("review kind title should be English again, got %v", first["title"])
	}
}
