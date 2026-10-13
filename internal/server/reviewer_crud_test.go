package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/config"
)

// reviewerFromMeta returns one reviewer as listed by GET /api/meta.
func (e *testEnv) reviewerFromMeta(name string) map[string]any {
	e.t.Helper()
	meta := e.expect("GET", "/api/meta", nil, http.StatusOK)
	reviewers, _ := meta["reviewers"].([]any)
	for _, raw := range reviewers {
		if info, _ := raw.(map[string]any); info["name"] == name {
			return info
		}
	}
	return nil
}

func shellReviewer(script string, extra ...string) map[string]any {
	command := append([]string{"/bin/sh", "-c", script, "sh"}, extra...)
	return map[string]any{"description": "test double", "command": append(command, "{prompt}")}
}

func TestReviewerCustomLifecycle(t *testing.T) {
	env := newTestEnv(t)

	invalid := map[string]map[string]any{
		"auto":        shellReviewer("true"),
		"Bad%20Name":  shellReviewer("true"),
		"no-prompt":   {"command": []string{"/bin/echo", "hi"}},
		"empty":       {"command": []string{}},
		"slot-first":  {"command": []string{"{model}", "{prompt}"}},
		"dash-model":  {"command": []string{"/bin/echo", "--model={model}", "{prompt}"}, "model": "-rf"},
		"space-agent": {"command": []string{"/bin/echo", "--agent={agent}", "{prompt}"}, "agent": "two words"},
		"short-time":  {"command": []string{"/bin/echo", "{prompt}"}, "timeout": "5s"},
	}
	for name, body := range invalid {
		env.expect("PUT", "/api/reviewers/"+name, body, http.StatusBadRequest)
	}

	body := shellReviewer(`printf 'XPLAN-OK'`)
	body["model"], body["timeout"] = "fast-1", "1h"
	body["command"] = []string{"/bin/sh", "-c", `printf 'XPLAN-OK'`, "sh", "--model={model}", "{prompt}"}
	saved := env.expect("PUT", "/api/reviewers/my-agent", body, http.StatusOK)
	reviewer, _ := saved["reviewer"].(map[string]any)
	if reviewer["builtin"] != false || reviewer["supports_model"] != true || reviewer["supports_agent"] != false {
		t.Fatalf("saved reviewer flags = %v", reviewer)
	}
	if reviewer["model"] != "fast-1" || reviewer["effective_timeout"] != "1h" {
		t.Fatalf("saved reviewer model/timeout = %v", reviewer)
	}
	if other := env.reviewerFromMeta("fake"); other["effective_timeout"] != "20m" {
		t.Fatalf("reviewers without a timeout use the global one: %v", other)
	}

	env.expect("POST", "/api/reviewers/my-agent/test", nil, http.StatusOK)
	if info := env.reviewerFromMeta("my-agent"); info["health"] == nil {
		t.Fatalf("tested reviewer should carry health: %v", info)
	}
	body["model"] = "fast-2"
	env.expect("PUT", "/api/reviewers/my-agent", body, http.StatusOK)
	if info := env.reviewerFromMeta("my-agent"); info["health"] != nil {
		t.Fatalf("changing a reviewer must forget its old health: %v", info)
	}

	env.expect("PATCH", "/api/config", map[string]any{"default_reviewer": "my-agent"}, http.StatusOK)
	env.expect("DELETE", "/api/reviewers/my-agent", nil, http.StatusConflict)
	env.expect("PATCH", "/api/config", map[string]any{"default_reviewer": "auto", "reviewer_order": []string{"my-agent", "fake"}}, http.StatusOK)
	env.expect("DELETE", "/api/reviewers/my-agent", nil, http.StatusOK)
	if info := env.reviewerFromMeta("my-agent"); info != nil {
		t.Fatalf("deleted reviewer still listed: %v", info)
	}
	if order := env.app.Config().ReviewerOrder; slices.Contains(order, "my-agent") {
		t.Fatalf("deleted reviewer still in reviewer_order: %v", order)
	}
	env.expect("DELETE", "/api/reviewers/my-agent", nil, http.StatusBadRequest)
}

func TestReviewerModelAndAgentSlots(t *testing.T) {
	env := newTestEnv(t)
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("XPLAN_ARGS_FILE", argsFile)
	script := `printf '%s\n' "$@" > "$XPLAN_ARGS_FILE"; printf 'XPLAN-OK'`
	body := map[string]any{
		"command": []string{"/bin/sh", "-c", script, "sh", "--model={model}", "--agent={agent}", "{prompt}"},
		"model":   "provider/model-x",
		"agent":   "reviewer",
	}
	readArgs := func() []string {
		raw, err := os.ReadFile(argsFile)
		if err != nil {
			t.Fatalf("read args: %v", err)
		}
		return strings.Split(string(raw), "\n")
	}

	env.expect("PUT", "/api/reviewers/slots", body, http.StatusOK)
	env.expect("POST", "/api/reviewers/slots/test", nil, http.StatusOK)
	args := readArgs()
	if !slices.Contains(args, "--model=provider/model-x") || !slices.Contains(args, "--agent=reviewer") {
		t.Fatalf("model and agent should be passed: %q", args)
	}

	body["model"], body["agent"] = "", ""
	env.expect("PUT", "/api/reviewers/slots", body, http.StatusOK)
	env.expect("POST", "/api/reviewers/slots/test", nil, http.StatusOK)
	for _, arg := range readArgs() {
		if strings.HasPrefix(arg, "--model") || strings.HasPrefix(arg, "--agent") {
			t.Fatalf("empty slots must drop their argument, got %q", arg)
		}
	}
}

func TestBuiltinReviewerOverrides(t *testing.T) {
	env := newTestEnv(t)

	opencode := env.reviewerFromMeta("opencode")
	if opencode["builtin"] != true || opencode["agent"] != "plan" || opencode["modified"] != false {
		t.Fatalf("opencode preset = %v", opencode)
	}
	command, _ := opencode["command"].([]any)
	env.expect("PUT", "/api/reviewers/opencode", map[string]any{"command": command, "model": "openrouter/some-model", "agent": ""}, http.StatusOK)
	opencode = env.reviewerFromMeta("opencode")
	if opencode["agent"] != "plan" || opencode["model"] != "openrouter/some-model" || opencode["modified"] != true {
		t.Fatalf("an empty agent must fall back to the read-only preset agent: %v", opencode)
	}

	raw, err := os.ReadFile(filepath.Join(env.app.Home, config.FileName))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var stored struct {
		Reviewers map[string]config.ReviewerDef `yaml:"reviewers"`
	}
	if err := yaml.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if got := stored.Reviewers["opencode"]; got.Model != "openrouter/some-model" || len(got.Command) != 0 || got.Agent != "" {
		t.Fatalf("built-ins should be stored as overrides only, got %+v", got)
	}
	if _, ok := stored.Reviewers["codex"]; ok {
		t.Fatalf("untouched presets should not be written: %+v", stored.Reviewers["codex"])
	}
	if _, ok := stored.Reviewers["fake"]; !ok {
		t.Fatal("custom reviewers must be stored in full")
	}

	reloaded, err := config.Load(env.app.Home)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Reviewers["opencode"]; got.Model != "openrouter/some-model" || got.Agent != "plan" || !slices.Equal(got.Command, config.BuiltinReviewers()["opencode"].Command) {
		t.Fatalf("reloaded opencode = %+v", got)
	}

	env.expect("DELETE", "/api/reviewers/opencode", nil, http.StatusOK)
	if opencode = env.reviewerFromMeta("opencode"); opencode["model"] != "" || opencode["modified"] != false {
		t.Fatalf("reset should restore the preset: %v", opencode)
	}

	meta := env.expect("GET", "/api/meta", nil, http.StatusOK)
	templates, _ := meta["reviewer_templates"].([]any)
	if len(templates) == 0 {
		t.Fatal("meta should offer reviewer templates")
	}
	if devin := env.reviewerFromMeta("devin"); devin == nil || devin["builtin"] != true || devin["supports_model"] != true {
		t.Fatalf("devin preset = %v", devin)
	}
}

func TestSilentReviewerFailures(t *testing.T) {
	env := newTestEnv(t)
	cases := map[string]struct{ script, status string }{
		// qodercli: exit 0, nothing on stdout, the reason on stderr.
		"quiet-logout": {`echo 'user not logged in. please run interactive mode and login' >&2`, "auth"},
		// codebuddy: exit 0 with the login error as the whole answer.
		"short-logout": {`printf 'Authentication required. Please use /login command to sign in\n'`, "auth"},
		"silent":       {`exit 0`, "error"},
	}
	for name, testCase := range cases {
		env.expect("PUT", "/api/reviewers/"+name, shellReviewer(testCase.script), http.StatusOK)
		result := env.expect("POST", "/api/reviewers/"+name+"/test", nil, http.StatusOK)
		health, _ := result["health"].(map[string]any)
		if health["status"] != testCase.status {
			t.Fatalf("%s: health = %v, want %s", name, health, testCase.status)
		}
		if detail, _ := health["detail"].(string); detail == "" {
			t.Fatalf("%s: health should explain the failure: %v", name, health)
		}
	}
}

func TestReviewTimeoutExplainsItself(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug
	_, err := env.app.UpdateConfig(func(cfg *config.Config) error {
		cfg.Reviewers["slow"] = config.ReviewerDef{Command: []string{"/bin/sh", "-c", "echo 'Working...' >&2; sleep 30", "sh", "{prompt}"}}
		cfg.ReviewTimeout = "1s"
		return nil
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}
	env.expect("PATCH", base, map[string]any{"reviewer": "slow"}, http.StatusOK)
	started := env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusAccepted)
	jobID, _ := started["id"].(string)
	job := waitForJob(t, env, base, jobID)
	message, _ := job["error"].(string)
	if job["status"] != "failed" || !strings.Contains(message, "No answer within 1s") || !strings.Contains(message, "Working...") {
		t.Fatalf("timeout should explain itself and keep the agent's last line: %v", job)
	}
}

func TestOldConfigUpgrades(t *testing.T) {
	home := t.TempDir()
	oldConfig := `default_reviewer: omp
reviewers:
  omp:
    description: omp print mode with read-only tools
    command: [omp, -p, --no-session, --cwd, "{path}", --tools, "read,grep,glob", "{prompt}"]
  codex:
    description: Codex exec in read-only sandbox
    command: [codex, exec, --sandbox, read-only, --skip-git-repo-check, -C, "{path}", --profile, mine, "{prompt}"]
  mine:
    description: my own agent
    command: [my-agent, "{prompt}"]
`
	writeFile(t, home, config.FileName, oldConfig)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	application, err := app.New(ctx, home, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	env := &testEnv{t: t, handler: New(application, nil), app: application}

	omp := env.reviewerFromMeta("omp")
	ompCommand, _ := omp["command"].([]any)
	if !slices.Contains(ompCommand, any("--no-skills")) || !slices.Contains(ompCommand, any("--model={model}")) || omp["modified"] != false {
		t.Fatalf("an old built-in command should upgrade to the current preset: %v", omp)
	}
	codex := env.reviewerFromMeta("codex")
	codexCommand, _ := codex["command"].([]any)
	if !slices.Contains(codexCommand, any("--profile")) || codex["modified"] != true {
		t.Fatalf("a user-changed built-in command must be kept: %v", codex)
	}
	if mine := env.reviewerFromMeta("mine"); mine == nil || mine["builtin"] != false {
		t.Fatalf("custom reviewer lost: %v", mine)
	}
}
