package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/config"
)

// fakeReport is what the fake reviewer agent prints: Markdown plus the
// trailing JSON block xplan parses.
const fakeReport = "# Report\n\nAll good mostly.\n\n```json\n" +
	`{"verdict":"attention","score":72,"summary":"Two gaps","findings":[{"severity":"high","title":"No tests","file":"main.go","detail":"d","suggestion":"s"},{"severity":"low","title":"Typo"}]}` +
	"\n```\n"

type testEnv struct {
	t       *testing.T
	handler http.Handler
	app     *app.App
	project string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir()
	projectDir := t.TempDir()
	writeFile(t, projectDir, "README.md", "# Demo\n\nA demo project used by tests.\n")
	writeFile(t, projectDir, "docs/spec.md", "# Spec\n\n- [x] login\n- [ ] export\n")
	writeFile(t, projectDir, "CHANGELOG.md", "# Changelog\n\n## [0.3.0] - 2026-10-01\n\n- first\n")
	writeFile(t, projectDir, ".env", "SECRET=1\n")
	runIn(t, projectDir, "git", "init", "-q")
	runIn(t, projectDir, "git", "-c", "user.email=t@t", "-c", "user.name=t", "add", "README.md", "docs", "CHANGELOG.md")
	runIn(t, projectDir, "git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "init")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	application, err := app.New(ctx, home, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	_, err = application.UpdateConfig(func(cfg *config.Config) error {
		cfg.Reviewers["fake"] = config.ReviewerDef{
			Description: "test double",
			Command:     []string{"/bin/sh", "-c", "sleep 0.3; printf '%s' \"$XPLAN_FAKE\"", "{prompt}"},
		}
		cfg.DefaultReviewer = "fake"
		cfg.Notify.Desktop = false // tests must not pop notifications on the developer's desktop
		return nil
	})
	if err != nil {
		t.Fatalf("configure fake reviewer: %v", err)
	}
	t.Setenv("XPLAN_FAKE", fakeReport)
	return &testEnv{t: t, handler: New(application, nil), app: application, project: projectDir}
}

// do sends a request as the UI would (loopback host + X-Xplan header).
func (e *testEnv) do(method, path string, body any) (int, map[string]any) {
	return e.doWith(method, path, body, "127.0.0.1:7788", true)
}

func (e *testEnv) doWith(method, path string, body any, host string, withHeader bool) (int, map[string]any) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	request := httptest.NewRequest(method, path, reader)
	request.Host = host
	if withHeader {
		request.Header.Set("X-Xplan", "1")
	}
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	decoded := map[string]any{}
	if recorder.Body.Len() > 0 {
		_ = json.Unmarshal(recorder.Body.Bytes(), &decoded)
	}
	return recorder.Code, decoded
}

func (e *testEnv) expect(method, path string, body any, want int) map[string]any {
	e.t.Helper()
	got, decoded := e.do(method, path, body)
	if got != want {
		e.t.Fatalf("%s %s: status %d, want %d (body %v)", method, path, got, want, decoded)
	}
	return decoded
}

func (e *testEnv) addProject() string {
	e.t.Helper()
	created := e.expect("POST", "/api/projects", map[string]string{"path": e.project, "name": "Demo"}, http.StatusCreated)
	slug, _ := created["slug"].(string)
	if slug == "" {
		e.t.Fatalf("created project has no slug: %v", created)
	}
	return slug
}

func TestSecurityGuard(t *testing.T) {
	env := newTestEnv(t)
	if code, _ := env.doWith("POST", "/api/projects", map[string]string{"path": env.project}, "127.0.0.1:7788", false); code != http.StatusForbidden {
		t.Fatalf("POST without X-Xplan: %d, want 403", code)
	}
	if code, _ := env.doWith("GET", "/api/meta", nil, "evil.example.com", true); code != http.StatusForbidden {
		t.Fatalf("foreign Host: %d, want 403", code)
	}
	if code, _ := env.doWith("GET", "/api/meta", nil, "localhost:7788", false); code != http.StatusOK {
		t.Fatalf("GET on localhost: %d, want 200", code)
	}
	env.expect("GET", "/api/nope", nil, http.StatusNotFound)
}

func TestMetaAndConfig(t *testing.T) {
	env := newTestEnv(t)
	meta := env.expect("GET", "/api/meta", nil, http.StatusOK)
	for _, key := range []string{"version", "statuses", "review_kinds", "reviewers", "commands", "doc_kinds"} {
		if _, ok := meta[key]; !ok {
			t.Fatalf("meta missing %q", key)
		}
	}
	env.expect("GET", "/api/config", nil, http.StatusOK)
	env.expect("PATCH", "/api/config", map[string]any{"scan_interval": "30m", "report_language": "Simplified Chinese"}, http.StatusOK)
	env.expect("PATCH", "/api/config", map[string]any{"scan_interval": "soon"}, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{"default_reviewer": "nobody"}, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{"default_schedule": map[string]string{"drift": "hourly"}}, http.StatusBadRequest)
}

func TestProjectLifecycle(t *testing.T) {
	env := newTestEnv(t)
	env.expect("POST", "/api/projects", map[string]string{"path": "/definitely/not/here"}, http.StatusBadRequest)
	slug := env.addProject()
	env.expect("POST", "/api/projects", map[string]string{"path": env.project}, http.StatusBadRequest)

	overview := env.expect("GET", "/api/overview", nil, http.StatusOK)
	cards, _ := overview["cards"].([]any)
	if len(cards) != 1 {
		t.Fatalf("overview cards = %d, want 1", len(cards))
	}
	card := cards[0].(map[string]any)
	for _, key := range []string{"health_level", "health", "checklist", "stack", "verdicts"} {
		if _, ok := card[key]; !ok {
			t.Fatalf("card missing %q", key)
		}
	}

	env.expect("GET", "/api/projects/"+slug, nil, http.StatusOK)
	env.expect("GET", "/api/projects/missing", nil, http.StatusNotFound)
	env.expect("PATCH", "/api/projects/"+slug, map[string]any{"status": "sleeping"}, http.StatusBadRequest)
	env.expect("PATCH", "/api/projects/"+slug, map[string]any{"reviewer": "ghost"}, http.StatusBadRequest)
	patched := env.expect("PATCH", "/api/projects/"+slug, map[string]any{"status": "waiting", "schedule": map[string]string{"code": "daily"}}, http.StatusOK)
	if patched["status"] != "waiting" {
		t.Fatalf("status not updated: %v", patched["status"])
	}

	brief := env.expect("GET", "/api/projects/"+slug+"/brief", nil, http.StatusOK)
	for _, key := range []string{"card", "intent", "since", "adrs", "latest_reviews", "active_reviews"} {
		if _, ok := brief[key]; !ok {
			t.Fatalf("brief missing %q", key)
		}
	}
	env.expect("POST", "/api/projects/"+slug+"/scan", nil, http.StatusOK)
	env.expect("POST", "/api/projects/"+slug+"/visit", nil, http.StatusOK)

	env.expect("DELETE", "/api/projects/"+slug, nil, http.StatusNoContent)
	env.expect("GET", "/api/projects/"+slug, nil, http.StatusNotFound)
}

func TestIntentADRJournal(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug

	intent := env.expect("GET", base+"/intent", nil, http.StatusOK)
	if intent["filled"] != false {
		t.Fatalf("fresh intent should not be filled")
	}
	saved := env.expect("PUT", base+"/intent", map[string]string{"content": "# Intent\n\n## Why (the problem)\n\nParallel work loses context.\n"}, http.StatusOK)
	if saved["filled"] != true {
		t.Fatalf("written intent should be filled")
	}

	env.expect("POST", base+"/adrs", map[string]string{"title": ""}, http.StatusBadRequest)
	created := env.expect("POST", base+"/adrs", map[string]string{"title": "Use files not a database", "context": "Agents read files."}, http.StatusCreated)
	adrID, _ := created["id"].(string)
	if adrID != "0001-use-files-not-a-database" {
		t.Fatalf("adr id = %q", adrID)
	}
	listed := env.expect("GET", base+"/adrs", nil, http.StatusOK)
	if adrs, _ := listed["adrs"].([]any); len(adrs) != 1 {
		t.Fatalf("adrs = %v", listed["adrs"])
	}
	env.expect("PUT", base+"/adrs/"+adrID, map[string]string{"content": "# 0001. Use files\n\n- Status: accepted\n"}, http.StatusOK)
	env.expect("PUT", base+"/adrs/not-an-adr", map[string]string{"content": "x"}, http.StatusNotFound)
	env.expect("PUT", base+"/adrs/9999-missing", map[string]string{"content": "x"}, http.StatusNotFound)

	env.expect("POST", base+"/journal", map[string]string{"kind": "park"}, http.StatusBadRequest)
	env.expect("POST", base+"/journal", map[string]string{"kind": "bogus", "text": "x"}, http.StatusBadRequest)
	park := env.expect("POST", base+"/journal", map[string]string{"kind": "park", "stopped": "login api", "next": "tests"}, http.StatusCreated)
	if park["head"] == "" {
		t.Fatalf("park note should record HEAD")
	}
	env.expect("POST", base+"/journal", map[string]string{"kind": "note", "text": "remember the cache"}, http.StatusCreated)
	journal := env.expect("GET", base+"/journal", nil, http.StatusOK)
	if entries, _ := journal["entries"].([]any); len(entries) != 2 {
		t.Fatalf("journal entries = %d, want 2", len(entries))
	}
}

func TestDocsAndFileSafety(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug

	docs := env.expect("GET", base+"/docs", nil, http.StatusOK)
	if list, _ := docs["docs"].([]any); len(list) != 3 {
		t.Fatalf("docs = %d, want 3 (README, spec, CHANGELOG)", len(list))
	}
	file := env.expect("GET", base+"/file?path=docs/spec.md", nil, http.StatusOK)
	if file["kind"] != "spec" {
		t.Fatalf("spec kind = %v", file["kind"])
	}
	env.expect("GET", base+"/file", nil, http.StatusBadRequest)
	env.expect("GET", base+"/file?path=.env", nil, http.StatusForbidden)
	env.expect("GET", base+"/file?path=../../../../etc/hosts", nil, http.StatusNotFound)
	env.expect("GET", base+"/file?path=nope.md", nil, http.StatusNotFound)
	tree := env.expect("GET", base+"/tree", nil, http.StatusOK)
	if entries, _ := tree["entries"].([]any); len(entries) == 0 {
		t.Fatalf("tree should list entries")
	}
}

func TestReviewLifecycle(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug

	env.expect("POST", base+"/reviews", map[string]string{"kind": "vibes"}, http.StatusBadRequest)
	env.expect("POST", base+"/reviews", map[string]string{"kind": "ask"}, http.StatusBadRequest)
	started := env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusAccepted)
	jobID, _ := started["id"].(string)
	env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusConflict)

	job := waitForJob(t, env, base, jobID)
	if job["status"] != "done" || job["verdict"] != "attention" {
		t.Fatalf("job = status %v verdict %v error %v", job["status"], job["verdict"], job["error"])
	}
	if findings, _ := job["findings"].([]any); len(findings) != 2 {
		t.Fatalf("findings = %v", job["findings"])
	}
	detail := env.expect("GET", base+"/reviews/"+jobID, nil, http.StatusOK)
	if report, _ := detail["report"].(string); report == "" || bytes.Contains([]byte(report), []byte("```json")) {
		t.Fatalf("report should be Markdown without the JSON block: %q", report)
	}
	env.expect("GET", base+"/reviews/20990101-000000-code-ffff", nil, http.StatusNotFound)
	env.expect("POST", base+"/reviews/"+jobID+"/cancel", nil, http.StatusBadRequest)

	inbox := env.expect("GET", "/api/inbox?min=medium", nil, http.StatusOK)
	if items, _ := inbox["items"].([]any); len(items) != 1 {
		t.Fatalf("inbox items = %v, want only the high finding", inbox["items"])
	}
	env.expect("GET", "/api/reviews/active", nil, http.StatusOK)
	list := env.expect("GET", base+"/reviews", nil, http.StatusOK)
	if jobs, _ := list["jobs"].([]any); len(jobs) != 1 {
		t.Fatalf("jobs = %d", len(jobs))
	}
}

func TestConsole(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug

	env.expect("POST", base+"/console", map[string]string{"input": "  "}, http.StatusBadRequest)
	help := env.expect("POST", base+"/console", map[string]string{"input": "/help"}, http.StatusOK)
	if help["ok"] != true {
		t.Fatalf("/help not ok: %v", help)
	}
	unknown := env.expect("POST", base+"/console", map[string]string{"input": "/frobnicate"}, http.StatusOK)
	if unknown["ok"] != false {
		t.Fatalf("unknown command should not be ok")
	}
	status := env.expect("POST", base+"/console", map[string]string{"input": "/status paused"}, http.StatusOK)
	if status["ok"] != true {
		t.Fatalf("/status paused: %v", status)
	}
	park := env.expect("POST", base+"/console", map[string]string{"input": "/park wired the brief | add tests | naming?"}, http.StatusOK)
	if park["ok"] != true {
		t.Fatalf("/park: %v", park)
	}
	ask := env.expect("POST", base+"/console", map[string]string{"input": "how is auth done?"}, http.StatusOK)
	if ask["job_id"] == "" || ask["job_id"] == nil {
		t.Fatalf("plain text should start an ask job: %v", ask)
	}
	waitForJob(t, env, base, ask["job_id"].(string))
	history := env.expect("GET", base+"/console", nil, http.StatusOK)
	if items, _ := history["items"].([]any); len(items) != 5 {
		t.Fatalf("history items = %d, want 5", len(items))
	}
	env.expect("POST", "/api/projects/missing/console", map[string]string{"input": "/help"}, http.StatusNotFound)
}

// fakeIntentAnswer mimics an agent that ignores "no preamble, no fence" and
// loosens the headings; the draft must still come out clean.
const fakeIntentAnswer = "Here is the draft you asked for:\n\n```markdown\n# Intent — whatever\n\n" +
	"## Why\n- Parallel projects lose their original idea.\n\n" +
	"## Goal:\n- One place that remembers why each project exists.\n\n" +
	"## Non-goals\n- Writing code into projects.\n\n## Sources\n- README.md\n```\n\nHope this helps."

func TestIntentDraft(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug
	t.Setenv("XPLAN_FAKE", fakeIntentAnswer)

	bulk := env.expect("POST", "/api/intent-drafts", nil, http.StatusAccepted)
	if queued, _ := bulk["queued"].([]any); len(queued) != 0 {
		t.Fatalf("a project started with its xplan entry must not be drafted in bulk: %v", bulk)
	}
	skipped, _ := bulk["skipped"].([]any)
	if len(skipped) != 1 || skipped[0].(map[string]any)["code"] != "new" {
		t.Fatalf("skipped = %v, want the new project with code new", bulk["skipped"])
	}

	// Backdate the root commit: the project now existed long before it was registered.
	old := []string{"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z"}
	runInEnv(t, env.project, old, "git", "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--amend", "--no-edit")
	env.expect("POST", base+"/scan", nil, http.StatusOK)
	bulk = env.expect("POST", "/api/intent-drafts", nil, http.StatusAccepted)
	queued, _ := bulk["queued"].([]any)
	if len(queued) != 1 {
		t.Fatalf("established project should be drafted: %v", bulk)
	}
	jobID, _ := queued[0].(map[string]any)["job_id"].(string)
	if job := waitForJob(t, env, base, jobID); job["status"] != "done" || job["intent_written"] != true {
		t.Fatalf("draft job = %v written=%v (%v)", job["status"], job["intent_written"], job["error"])
	}

	intent := env.expect("GET", base+"/intent", nil, http.StatusOK)
	content, _ := intent["content"].(string)
	if intent["filled"] != true || intent["draft"] != true {
		t.Fatalf("drafted intent should be filled and marked draft: %v", intent)
	}
	for _, want := range []string{"# Intent — Demo\n", "> **Draft** — inferred by fake", "## Why (the problem)", "## Goal (north star)", "## Sources"} {
		if !bytes.Contains([]byte(content), []byte(want)) {
			t.Fatalf("draft missing %q:\n%s", want, content)
		}
	}
	if bytes.Contains([]byte(content), []byte("Hope this helps")) || bytes.Contains([]byte(content), []byte("```")) {
		t.Fatalf("draft should drop the agent's preamble and fence:\n%s", content)
	}
	brief := env.expect("GET", base+"/brief", nil, http.StatusOK)
	card, _ := brief["card"].(map[string]any)
	if card["intent_draft"] != true || card["established"] != true || !hasFlag(card, "intent_draft") {
		t.Fatalf("card should flag the draft: %v", card["health"])
	}

	reviewed := "# Intent — Demo\n\n## Why (the problem)\n\nMy own words.\n"
	saved := env.expect("PUT", base+"/intent", map[string]string{"content": reviewed}, http.StatusOK)
	if saved["draft"] != false {
		t.Fatalf("intent without the marker is no longer a draft")
	}
	again := env.expect("POST", base+"/console", map[string]string{"input": "/draft-intent"}, http.StatusOK)
	againID, _ := again["job_id"].(string)
	if again["ok"] != true || againID == "" {
		t.Fatalf("/draft-intent: %v", again)
	}
	if job := waitForJob(t, env, base, againID); job["status"] != "done" || job["intent_written"] != nil {
		t.Fatalf("second draft = %v written=%v, want done and not written", job["status"], job["intent_written"])
	}
	if kept := env.expect("GET", base+"/intent", nil, http.StatusOK); kept["content"] != reviewed {
		t.Fatalf("a written intent must never be overwritten by a draft")
	}
	detail := env.expect("GET", base+"/reviews/"+againID, nil, http.StatusOK)
	if report, _ := detail["report"].(string); !bytes.Contains([]byte(report), []byte("> **Draft**")) {
		t.Fatalf("the kept draft should be the job report: %q", report)
	}

	t.Setenv("XPLAN_FAKE", "Sorry, I could not find anything.")
	failed := env.expect("POST", base+"/reviews", map[string]string{"kind": "intent"}, http.StatusAccepted)
	if job := waitForJob(t, env, base, failed["id"].(string)); job["status"] != "failed" {
		t.Fatalf("an answer without sections should fail, got %v", job["status"])
	}
	if review := env.expect("POST", base+"/console", map[string]string{"input": "/review intent"}, http.StatusOK); review["ok"] != false {
		t.Fatalf("/review must only accept review kinds: %v", review)
	}
}

func hasFlag(card map[string]any, code string) bool {
	flags, _ := card["health"].([]any)
	for _, flag := range flags {
		if entry, _ := flag.(map[string]any); entry["code"] == code {
			return true
		}
	}
	return false
}

func waitForJob(t *testing.T, env *testEnv, base, jobID string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		detail := env.expect("GET", base+"/reviews/"+jobID, nil, http.StatusOK)
		job, _ := detail["job"].(map[string]any)
		if status := job["status"]; status != "queued" && status != "running" {
			return job
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job %s did not finish", jobID)
	return nil
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runIn(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	runInEnv(t, dir, nil, name, args...)
}

func runInEnv(t *testing.T, dir string, env []string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
}
