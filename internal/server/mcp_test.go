package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ffqa/xpier-xplan/internal/mcp"
)

var mcpToolNames = []string{
	"xplan_projects", "xplan_context", "xplan_park", "xplan_note",
	"xplan_propose_adr", "xplan_inbox", "xplan_start_review", "xplan_get_review",
}

// mcpPost sends a raw MCP request the way an agent would: loopback host, no
// X-Xplan header.
func (e *testEnv) mcpPost(method, body string, headers map[string]string) *httptest.ResponseRecorder {
	e.t.Helper()
	request := httptest.NewRequest(method, mcp.Path, strings.NewReader(body))
	request.Host = "127.0.0.1:7788"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	e.handler.ServeHTTP(recorder, request)
	return recorder
}

// mcpRPC sends one request and returns its decoded JSON-RPC response.
func (e *testEnv) mcpRPC(method string, params any) map[string]any {
	e.t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 7, "method": method, "params": params})
	recorder := e.mcpPost(http.MethodPost, string(raw), nil)
	if recorder.Code != http.StatusOK {
		e.t.Fatalf("%s: status %d, body %s", method, recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		e.t.Fatalf("%s: content type %q", method, contentType)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil {
		e.t.Fatalf("%s: decode %s: %v", method, recorder.Body.String(), err)
	}
	if decoded["jsonrpc"] != "2.0" || decoded["id"] != float64(7) {
		e.t.Fatalf("%s: bad envelope %v", method, decoded)
	}
	return decoded
}

// mcpTool calls a tool and returns its text and whether it reported an error.
func (e *testEnv) mcpTool(name string, arguments map[string]any) (string, bool) {
	e.t.Helper()
	reply := e.mcpRPC("tools/call", map[string]any{"name": name, "arguments": arguments})
	result, ok := reply["result"].(map[string]any)
	if !ok {
		e.t.Fatalf("%s: no result in %v", name, reply)
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		e.t.Fatalf("%s: content = %v", name, result["content"])
	}
	item, _ := content[0].(map[string]any)
	if item["type"] != "text" {
		e.t.Fatalf("%s: content type %v", name, item["type"])
	}
	text, _ := item["text"].(string)
	return text, result["isError"] == true
}

func (e *testEnv) mustTool(name string, arguments map[string]any, wants ...string) string {
	e.t.Helper()
	text, isError := e.mcpTool(name, arguments)
	if isError {
		e.t.Fatalf("%s(%v) failed: %s", name, arguments, text)
	}
	for _, want := range wants {
		if !strings.Contains(text, want) {
			e.t.Fatalf("%s: missing %q in:\n%s", name, want, text)
		}
	}
	return text
}

func (e *testEnv) toolFails(name string, arguments map[string]any, want string) {
	e.t.Helper()
	text, isError := e.mcpTool(name, arguments)
	if !isError || !strings.Contains(text, want) {
		e.t.Fatalf("%s(%v): want a tool error containing %q, got isError=%v %q", name, arguments, want, isError, text)
	}
}

func rpcErrorCode(reply map[string]any) float64 {
	failure, _ := reply["error"].(map[string]any)
	code, _ := failure["code"].(float64)
	return code
}

func TestMCPProtocol(t *testing.T) {
	env := newTestEnv(t)

	initialized := env.mcpRPC("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	})
	result, _ := initialized["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocol version = %v, want the client's", result["protocolVersion"])
	}
	if info, _ := result["serverInfo"].(map[string]any); info["name"] != "xplan" {
		t.Fatalf("serverInfo = %v", result["serverInfo"])
	}
	if capabilities, _ := result["capabilities"].(map[string]any); capabilities["tools"] == nil {
		t.Fatalf("capabilities = %v", result["capabilities"])
	}
	if instructions, _ := result["instructions"].(string); !strings.Contains(instructions, "read-only for agents") {
		t.Fatalf("instructions = %q", instructions)
	}
	future := env.mcpRPC("initialize", map[string]any{"protocolVersion": "2099-01-01"})
	if got := future["result"].(map[string]any)["protocolVersion"]; got != mcp.SupportedVersions[0] {
		t.Fatalf("unknown version answered with %v, want %s", got, mcp.SupportedVersions[0])
	}

	notification := env.mcpPost(http.MethodPost, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, nil)
	if notification.Code != http.StatusAccepted || notification.Body.Len() != 0 {
		t.Fatalf("notification: %d %q, want 202 with no body", notification.Code, notification.Body.String())
	}
	if pong := env.mcpRPC("ping", nil); pong["result"] == nil {
		t.Fatalf("ping = %v", pong)
	}

	listed := env.mcpRPC("tools/list", map[string]any{})
	tools, _ := listed["result"].(map[string]any)["tools"].([]any)
	names := []string{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		names = append(names, tool["name"].(string))
		schema, _ := tool["inputSchema"].(map[string]any)
		if schema["type"] != "object" || tool["description"] == "" {
			t.Fatalf("tool %v has no object schema or description", tool["name"])
		}
	}
	if !slices.Equal(names, mcpToolNames) {
		t.Fatalf("tools = %v, want %v (no tool may write the intent)", names, mcpToolNames)
	}

	if code := rpcErrorCode(env.mcpRPC("resources/list", nil)); code != -32601 {
		t.Fatalf("unknown method code = %v, want -32601", code)
	}
	if code := rpcErrorCode(env.mcpRPC("tools/call", map[string]any{"name": "xplan_write_intent"})); code != -32602 {
		t.Fatalf("unknown tool code = %v, want -32602", code)
	}

	batch := env.mcpPost(http.MethodPost, `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"}]`, nil)
	var replies []map[string]any
	if err := json.Unmarshal(batch.Body.Bytes(), &replies); batch.Code != http.StatusOK || err != nil || len(replies) != 1 {
		t.Fatalf("batch: %d %s", batch.Code, batch.Body.String())
	}
	broken := env.mcpPost(http.MethodPost, `{"jsonrpc":`, nil)
	if broken.Code != http.StatusBadRequest || !strings.Contains(broken.Body.String(), "-32700") {
		t.Fatalf("invalid JSON: %d %s", broken.Code, broken.Body.String())
	}
	stream := env.mcpPost(http.MethodGet, "", nil)
	if stream.Code != http.StatusMethodNotAllowed || stream.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET: %d allow=%q, want 405 Allow: POST", stream.Code, stream.Header().Get("Allow"))
	}
	if old := env.mcpPost(http.MethodPost, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, map[string]string{"MCP-Protocol-Version": "1900-01-01"}); old.Code != http.StatusBadRequest {
		t.Fatalf("unsupported protocol header: %d, want 400", old.Code)
	}
}

func TestMCPSecurity(t *testing.T) {
	env := newTestEnv(t)
	ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	if code := env.mcpPost(http.MethodPost, ping, map[string]string{"Origin": "https://evil.example"}).Code; code != http.StatusForbidden {
		t.Fatalf("foreign Origin: %d, want 403", code)
	}
	if code := env.mcpPost(http.MethodPost, ping, map[string]string{"Origin": "http://localhost:5173"}).Code; code != http.StatusOK {
		t.Fatalf("loopback Origin: %d, want 200", code)
	}
	request := httptest.NewRequest(http.MethodPost, mcp.Path, strings.NewReader(ping))
	request.Host = "evil.example.com"
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("foreign Host: %d, want 403", recorder.Code)
	}
}

func TestMCPProjectMemory(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug
	intent := "# Demo intent\n\n## Why (the problem)\n\nParallel work loses context.\n\n## Goal\n\n- Keep every project on track\n"
	env.expect("PUT", base+"/intent", map[string]string{"content": intent}, http.StatusOK)
	inside := filepath.Join(env.project, "docs")

	env.mustTool("xplan_projects", nil, "**Demo**", "`"+slug+"`", env.project)
	env.mustTool("xplan_projects", map[string]any{"status": "done"}, "No done projects")
	env.mustTool("xplan_context", map[string]any{"path": inside}, "# Demo: project context", "Keep every project on track", "read-only for agents")
	env.mustTool("xplan_context", map[string]any{"project": "demo"}, "# Demo: project context")
	env.toolFails("xplan_context", map[string]any{"path": t.TempDir()}, "no xplan project contains")
	env.toolFails("xplan_context", map[string]any{"path": "docs"}, "must be absolute")
	env.toolFails("xplan_context", nil, "pass `path`")
	env.toolFails("xplan_context", map[string]any{"project": "nope"}, "registered: "+slug)

	env.mustTool("xplan_park", map[string]any{"path": inside, "stopped": "MCP tools", "next": "settings card"}, "Park note saved for Demo")
	env.toolFails("xplan_park", map[string]any{"path": inside}, "at least one field")
	parks := env.expect("GET", base+"/journal?kind=park", nil, http.StatusOK)
	if entries, _ := parks["entries"].([]any); len(entries) != 1 || entries[0].(map[string]any)["next"] != "settings card" {
		t.Fatalf("park entries = %v", parks["entries"])
	}

	env.mustTool("xplan_note", map[string]any{"path": inside, "text": "The cache key changed"}, "Note saved")
	env.toolFails("xplan_note", map[string]any{"path": inside, "text": strings.Repeat("字", 4001)}, "keep it under 4000")
	env.toolFails("xplan_note", map[string]any{"path": inside, "text": "  "}, "text is required")

	env.mustTool("xplan_propose_adr", map[string]any{
		"path": inside, "title": "Serve MCP from the xplan server", "context": "Agents need project memory.",
		"decision": "Mount /mcp in serve.\n- Status: accepted", "consequences": "One writer process.",
	}, "Proposed ADR 0001", "status: proposed")
	env.toolFails("xplan_propose_adr", map[string]any{"path": inside, "title": "No decision"}, "decision is required")
	adrs := env.expect("GET", base+"/adrs", nil, http.StatusOK)["adrs"].([]any)
	adr := adrs[0].(map[string]any)
	content, _ := adr["content"].(string)
	if adr["status"] != "proposed" || !strings.Contains(content, "Mount /mcp in serve.") || !strings.Contains(content, "Proposed by: an agent") || !strings.Contains(content, "One writer process.") {
		t.Fatalf("proposed ADR = %v", adr)
	}

	env.mustTool("xplan_context", map[string]any{"path": inside},
		"Serve MCP from the xplan server (proposed)", "Decision: Mount /mcp in serve.",
		"## Where work stopped", "- Next: settings card", "The cache key changed")
	if saved := env.expect("GET", base+"/intent", nil, http.StatusOK); saved["content"] != intent {
		t.Fatalf("intent changed through MCP: %q", saved["content"])
	}
}

func TestMCPReviews(t *testing.T) {
	env := newTestEnv(t)
	slug := env.addProject()
	base := "/api/projects/" + slug

	env.toolFails("xplan_start_review", map[string]any{"path": env.project, "kind": "intent"}, "kind must be one of")
	env.toolFails("xplan_start_review", map[string]any{"path": env.project, "kind": "ask"}, "question is required")
	env.toolFails("xplan_get_review", map[string]any{"path": env.project}, "no reviews yet")
	started := env.mustTool("xplan_start_review", map[string]any{"path": env.project, "kind": "code"}, "Started a code review of Demo")
	match := regexp.MustCompile(`\(id (\S+), reviewer fake\)`).FindStringSubmatch(started)
	if match == nil {
		t.Fatalf("no job id in %q", started)
	}
	jobID := match[1]
	env.mustTool("xplan_start_review", map[string]any{"path": env.project, "kind": "code"}, "already running (id "+jobID+")")
	job := waitForJob(t, env, base, jobID)
	if job["trigger"] != "mcp" || job["status"] != "done" {
		t.Fatalf("job = %v", job)
	}

	env.mustTool("xplan_get_review", map[string]any{"path": env.project, "id": jobID}, "Verdict: attention · Score: 72", "[high] code: No tests (main.go)", "## Report", "All good mostly.")
	env.mustTool("xplan_get_review", map[string]any{"project": slug, "kind": "code"}, "(id "+jobID+")")
	env.toolFails("xplan_get_review", map[string]any{"path": env.project, "kind": "security"}, "no security review yet")

	inbox := env.mustTool("xplan_inbox", nil, "1 open findings at high or above", "Demo · code: No tests")
	if strings.Contains(inbox, "Typo") {
		t.Fatalf("default inbox should skip low findings:\n%s", inbox)
	}
	env.mustTool("xplan_inbox", map[string]any{"path": env.project, "min_severity": "low"}, "[low] code: Typo")
	env.toolFails("xplan_inbox", map[string]any{"min_severity": "urgent"}, "unknown severity")
	env.mustTool("xplan_context", map[string]any{"path": env.project}, "- code: attention (72)", "## Open findings (medium and above, 1)")

	info := env.expect("GET", "/api/mcp", nil, http.StatusOK)
	if url, _ := info["url"].(string); url != "http://127.0.0.1:7788/mcp" {
		t.Fatalf("mcp url = %q", url)
	}
	listed, _ := info["tools"].([]any)
	if len(listed) != len(mcpToolNames) || listed[1].(map[string]any)["read_only"] != true || listed[2].(map[string]any)["read_only"] != false {
		t.Fatalf("mcp tools = %v", listed)
	}
}
