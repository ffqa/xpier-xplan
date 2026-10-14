package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// webhookSink records the payloads posted to a fake chat webhook. Requests to
// /rejected answer like Feishu does when a signature check fails.
type webhookSink struct {
	server   *httptest.Server
	payloads chan map[string]any
}

func newWebhookSink(t *testing.T) *webhookSink {
	t.Helper()
	sink := &webhookSink{payloads: make(chan map[string]any, 10)}
	sink.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/rejected") {
			_, _ = w.Write([]byte(`{"code":19021,"msg":"sign match fail"}`))
			return
		}
		payload := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		sink.payloads <- payload
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(sink.server.Close)
	return sink
}

func (s *webhookSink) next(t *testing.T) map[string]any {
	t.Helper()
	select {
	case payload := <-s.payloads:
		return payload
	case <-time.After(5 * time.Second):
		t.Fatal("no webhook call within 5s")
		return nil
	}
}

func (s *webhookSink) none(t *testing.T) {
	t.Helper()
	select {
	case payload := <-s.payloads:
		t.Fatalf("unexpected webhook call: %v", payload)
	case <-time.After(700 * time.Millisecond):
	}
}

func TestNotifications(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug
	sink := newWebhookSink(t)
	secretURL := sink.server.URL + "/hook/secret-token-123"

	settings := env.expect("GET", "/api/config", nil, http.StatusOK)["notify"].(map[string]any)
	if settings["min_verdict"] != "attention" || settings["webhook_set"] != false {
		t.Fatalf("default notify settings = %v", settings)
	}
	env.expect("POST", "/api/notify/test", nil, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"webhook_url": "ftp://example.com/x"}}, http.StatusBadRequest)
	env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"min_verdict": "meh"}}, http.StatusBadRequest)

	updated := env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"webhook_url": secretURL}}, http.StatusOK)
	view := updated["notify"].(map[string]any)
	if view["webhook_set"] != true || strings.Contains(view["webhook_hint"].(string), "secret-token") {
		t.Fatalf("webhook should be set and masked: %v", view)
	}
	if raw := env.rawBody("GET", "/api/config"); strings.Contains(raw, "secret-token") {
		t.Fatal("GET /api/config must never return the webhook URL")
	}
	if stored, _ := os.ReadFile(filepath.Join(env.app.Home, "config.yaml")); !strings.Contains(string(stored), "secret-token-123") {
		t.Fatal("the webhook URL should be saved in config.yaml")
	}

	tested := env.expect("POST", "/api/notify/test", nil, http.StatusOK)
	if results := tested["results"].([]any); len(results) != 1 || results[0].(map[string]any)["ok"] != true {
		t.Fatalf("test notification results = %v", tested)
	}
	if payload := sink.next(t); payload["kind"] != "test" {
		t.Fatalf("test payload = %v", payload)
	}

	started := env.expect("POST", base+"/reviews", map[string]string{"kind": "code"}, http.StatusAccepted)
	jobID := started["id"].(string)
	waitForJob(t, env, base, jobID)
	payload := sink.next(t)
	if payload["project"] != slug || payload["verdict"] != "attention" || !strings.Contains(payload["url"].(string), "/#/p/"+slug+"/reviews/"+jobID) {
		t.Fatalf("review payload = %v", payload)
	}

	env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"min_verdict": "critical"}}, http.StatusOK)
	quiet := env.expect("POST", base+"/reviews", map[string]string{"kind": "security"}, http.StatusAccepted)
	waitForJob(t, env, base, quiet["id"].(string))
	sink.none(t)

	t.Setenv("XPLAN_FAKE", "")
	failed := env.expect("POST", base+"/reviews", map[string]string{"kind": "test"}, http.StatusAccepted)
	if job := waitForJob(t, env, base, failed["id"].(string)); job["status"] != "failed" {
		t.Fatalf("a silent agent should fail the review, got %v", job["status"])
	}
	if payload := sink.next(t); payload["status"] != "failed" {
		t.Fatalf("failed reviews must always notify: %v", payload)
	}

	env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"webhook_url": sink.server.URL + "/rejected"}}, http.StatusOK)
	rejected := env.expect("POST", "/api/notify/test", nil, http.StatusOK)["results"].([]any)[0].(map[string]any)
	if rejected["ok"] != false || !strings.Contains(rejected["error"].(string), "19021") {
		t.Fatalf("a rejection inside a 200 answer must be reported: %v", rejected)
	}

	cleared := env.expect("PATCH", "/api/config", map[string]any{"notify": map[string]any{"webhook_url": ""}}, http.StatusOK)
	if cleared["notify"].(map[string]any)["webhook_set"] != false {
		t.Fatalf("an empty webhook_url should remove the webhook: %v", cleared["notify"])
	}
}

// rawBody returns the undecoded response body of a UI-style request.
func (e *testEnv) rawBody(method, path string) string {
	e.t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.Host = "127.0.0.1:7788"
	request.Header.Set("X-Xplan", "1")
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	return recorder.Body.String()
}
