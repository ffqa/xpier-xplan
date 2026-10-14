// Package mcp exposes xplan's project memory to coding agents over the Model
// Context Protocol (Streamable HTTP transport, JSON responses, no sessions).
//
// Agents can read a project's intent, decisions, park notes and review
// findings, and write park notes, notes and proposed decisions. No tool writes
// the intent: that stays the owner's.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/app"
)

// Path is where the server mounts the MCP endpoint.
const Path = "/mcp"

// SupportedVersions lists protocol revisions xplan speaks, newest first.
var SupportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const maxRequestBytes = 1 << 20

// JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Handler answers MCP requests for one xplan instance.
type Handler struct {
	app    *app.App
	logger *log.Logger
	tools  []Tool
	byName map[string]Tool
}

// New builds the handler with xplan's tool set.
func New(application *app.App) *Handler {
	handler := &Handler{app: application, logger: application.Logger, byName: map[string]Tool{}}
	handler.tools = handler.toolset()
	for _, tool := range handler.tools {
		handler.byName[tool.Name] = tool
	}
	return handler
}

// Tools lists the tools as clients see them (used by the settings page).
func (h *Handler) Tools() []Tool { return slices.Clone(h.tools) }

// ServeHTTP implements the Streamable HTTP transport without server-initiated
// streams: POST carries JSON-RPC, GET and DELETE are not offered.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !AllowedOrigin(r.Header.Get("Origin")) {
		http.Error(w, "xplan only accepts MCP requests from local clients", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "use POST: xplan does not open server-sent event streams", http.StatusMethodNotAllowed)
		return
	}
	if version := r.Header.Get("MCP-Protocol-Version"); version != "" && !slices.Contains(SupportedVersions, version) {
		http.Error(w, fmt.Sprintf("unsupported MCP protocol version %q", version), http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeRPC(w, http.StatusRequestEntityTooLarge, errorResponse(nil, codeInvalidRequest, "request body too large"))
		return
	}
	replies, status := h.handleBody(r.Context(), body)
	if replies == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeRPC(w, status, replies)
}

// handleBody processes a single message or a batch. A nil reply means the
// body held only notifications or responses (answered with 202).
func (h *Handler) handleBody(ctx context.Context, body []byte) (any, int) {
	body = bytes.TrimSpace(body)
	if len(body) > 0 && body[0] == '[' {
		var batch []message
		if err := json.Unmarshal(body, &batch); err != nil {
			return errorResponse(nil, codeParseError, "invalid JSON"), http.StatusBadRequest
		}
		replies := []response{}
		for _, item := range batch {
			if reply := h.handleMessage(ctx, item); reply != nil {
				replies = append(replies, *reply)
			}
		}
		if len(replies) == 0 {
			return nil, http.StatusAccepted
		}
		return replies, http.StatusOK
	}
	var single message
	if err := json.Unmarshal(body, &single); err != nil {
		return errorResponse(nil, codeParseError, "invalid JSON"), http.StatusBadRequest
	}
	if reply := h.handleMessage(ctx, single); reply != nil {
		return reply, http.StatusOK
	}
	return nil, http.StatusAccepted
}

// handleMessage returns nil for notifications and client responses.
func (h *Handler) handleMessage(ctx context.Context, msg message) *response {
	if msg.Method == "" {
		return nil
	}
	isNotification := len(msg.ID) == 0
	if msg.JSONRPC != "2.0" {
		if isNotification {
			return nil
		}
		return errorResponse(msg.ID, codeInvalidRequest, `jsonrpc must be "2.0"`)
	}
	result, failure := h.dispatch(ctx, msg)
	if isNotification {
		return nil
	}
	if failure != nil {
		return &response{JSONRPC: "2.0", ID: msg.ID, Error: failure}
	}
	return &response{JSONRPC: "2.0", ID: msg.ID, Result: result}
}

func (h *Handler) dispatch(ctx context.Context, msg message) (any, *rpcError) {
	switch msg.Method {
	case "initialize":
		return h.initialize(msg.Params)
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": h.tools}, nil
	case "tools/call":
		return h.callTool(ctx, msg.Params)
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + msg.Method}
	}
}

func (h *Handler) initialize(params json.RawMessage) (any, *rpcError) {
	var request struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid initialize params"}
		}
	}
	version := SupportedVersions[0]
	if slices.Contains(SupportedVersions, request.ProtocolVersion) {
		version = request.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "xplan", "title": "xplan", "version": app.Version},
		"instructions":    serverInstructions,
	}, nil
}

const serverInstructions = `xplan keeps the owner's record of each project: why it exists (intent), the decisions made (ADRs), where work last stopped (park notes) and what reviewers found.
Call xplan_context with your working directory when a session starts. The intent is read-only for agents: if the task conflicts with it, stop and ask the owner instead of working around it.
Before you stop, record where you are with xplan_park. Use xplan_note for things the owner should read later and xplan_propose_adr for decisions; the owner accepts or rejects proposals.
This is project memory, not a general-purpose store: do not save personal data, secrets or chat transcripts here.`

func (h *Handler) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var request struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid tools/call params"}
	}
	tool, ok := h.byName[request.Name]
	if !ok {
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool: " + request.Name}
	}
	text, err := h.runTool(ctx, tool, request.Arguments)
	if err != nil {
		return toolResult(err.Error(), true), nil
	}
	return toolResult(text, false), nil
}

// runTool turns a panic inside a tool into a tool error so one bad call
// cannot take the whole request down.
func (h *Handler) runTool(ctx context.Context, tool Tool, arguments json.RawMessage) (text string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			h.logger.Printf("mcp: tool %s panicked: %v", tool.Name, recovered)
			err = errors.New("internal error in " + tool.Name)
		}
	}()
	text, err = tool.run(ctx, arguments)
	if err != nil {
		h.logger.Printf("mcp: %s failed: %v", tool.Name, err)
	}
	return text, err
}

func toolResult(text string, isError bool) map[string]any {
	result := map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
	if isError {
		result["isError"] = true
	}
	return result
}

func errorResponse(id json.RawMessage, code int, text string) *response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: text}}
}

func writeRPC(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("mcp: encode response: %v", err)
	}
}

// AllowedOrigin accepts requests without an Origin (CLI and desktop clients)
// and from loopback pages; a browser tab on another site is refused, which
// blocks DNS-rebinding and cross-site requests.
func AllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.Trim(parsed.Hostname(), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
