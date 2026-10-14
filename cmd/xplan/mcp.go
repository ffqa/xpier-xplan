package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/mcp"
)

// bridgeTimeout covers the slowest tool call (xplan_context rescans git).
const bridgeTimeout = 2 * time.Minute

// cmdMCP connects an MCP client that only speaks stdio to the running xplan:
// each JSON-RPC line on stdin is posted to /mcp and the reply is written to
// stdout. Records stay in one process, the server, so nothing races.
func cmdMCP(ctx context.Context, application *app.App, args []string) error {
	flags := flag.NewFlagSet("mcp", flag.ExitOnError)
	endpoint := flags.String("url", application.BaseURL()+mcp.Path, "MCP endpoint of the running xplan")
	if err := flags.Parse(args); err != nil {
		return err
	}
	bridge := &stdioBridge{endpoint: *endpoint, client: &http.Client{Timeout: bridgeTimeout}}
	return bridge.run(ctx, os.Stdin, os.Stdout)
}

type stdioBridge struct {
	endpoint string
	client   *http.Client
}

func (b *stdioBridge) run(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		reply, err := b.forward(ctx, line)
		if err != nil {
			reply = unreachableReply(line, fmt.Errorf("xplan is not reachable at %s (start it with `xplan service install` or `xplan serve`): %w", b.endpoint, err))
		}
		if len(reply) == 0 {
			continue
		}
		if _, err := out.Write(append(bytes.TrimSpace(reply), '\n')); err != nil {
			return fmt.Errorf("write to MCP client: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read from MCP client: %w", err)
	}
	return nil
}

// forward posts one message; an empty reply means the server accepted a
// notification (202).
func (b *stdioBridge) forward(ctx context.Context, line []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint, bytes.NewReader(line))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if len(bytes.TrimSpace(body)) == 0 || !json.Valid(body) {
		return nil, fmt.Errorf("status %d: %s", response.StatusCode, bytes.TrimSpace(body))
	}
	return body, nil
}

// unreachableReply answers a request with a JSON-RPC error so the client
// shows why instead of hanging; notifications get no reply.
func unreachableReply(line []byte, cause error) []byte {
	var msg struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(line, &msg) != nil || len(msg.ID) == 0 {
		return nil
	}
	reply, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"error":   map[string]any{"code": -32603, "message": cause.Error()},
	})
	if err != nil {
		return nil
	}
	return reply
}
