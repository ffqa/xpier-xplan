// Package console interprets the command line at the bottom of the UI.
// Plain text becomes a read-only question to the project's reviewer agent;
// slash commands act on the project.
package console

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ffqa/xplan/internal/app"
	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/review"
)

// Result is what a command returns to the UI.
type Result struct {
	OK      bool     `json:"ok"`
	Output  string   `json:"output"`
	JobID   string   `json:"job_id,omitempty"`
	JobIDs  []string `json:"job_ids,omitempty"`
	Refresh []string `json:"refresh,omitempty"`
}

// HistoryItem is one line of commands.jsonl.
type HistoryItem struct {
	At     time.Time `json:"at"`
	Input  string    `json:"input"`
	Result Result    `json:"result"`
}

// Command describes a slash command for help and autocomplete.
type Command struct {
	Name    string `json:"name"`
	Args    string `json:"args"`
	Summary string `json:"summary"`
}

// Commands is the catalogue shown by /help and the UI autocomplete.
var Commands = []Command{
	{"/review", "<drift|code|security|deploy|test|all>", "Run a read-only external review"},
	{"/ask", "<question>", "Ask the reviewer agent about this project (plain text does the same)"},
	{"/park", "<where you stopped> [| next step] [| open questions]", "Leave a park note before switching away"},
	{"/note", "<text>", "Add a note to the journal"},
	{"/adr", "<title>", "Create a decision record"},
	{"/status", "<active|waiting|blocked|paused|done>", "Move the project on the board"},
	{"/scan", "", "Rescan docs, git and progress now"},
	{"/caught-up", "", "Mark everything as seen"},
	{"/xteam", "<message> | status", "Ring the project's xteam PM, or show xteam status"},
	{"/open", "", "Open the project in Cursor"},
	{"/help", "", "List commands"},
}

const externalTimeout = 60 * time.Second

// Console executes commands against the app.
type Console struct {
	App *app.App
}

// Run executes one input line for a project and records it in history.
func (c *Console) Run(slug, input string) (Result, error) {
	target, err := c.App.Store.Get(slug)
	if err != nil {
		return Result{}, err
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return Result{}, fmt.Errorf("%w: empty command", project.ErrInvalid)
	}
	result := c.dispatch(target, input)
	c.record(slug, input, result)
	return result, nil
}

func (c *Console) dispatch(target *project.Project, input string) Result {
	if !strings.HasPrefix(input, "/") {
		return c.ask(target, input)
	}
	name, args, _ := strings.Cut(input, " ")
	args = strings.TrimSpace(args)
	switch strings.ToLower(name) {
	case "/help", "/?":
		return Result{OK: true, Output: helpText()}
	case "/review":
		return c.review(target, args)
	case "/ask":
		return c.ask(target, args)
	case "/park":
		return c.park(target, args)
	case "/note":
		return c.note(target, args)
	case "/adr":
		return c.adr(target, args)
	case "/status":
		return c.status(target, args)
	case "/scan":
		return c.scan(target)
	case "/caught-up":
		if _, err := c.App.MarkCaughtUp(target.Slug); err != nil {
			return fail(err)
		}
		return Result{OK: true, Output: "Marked as caught up.", Refresh: []string{"brief", "overview"}}
	case "/xteam":
		return c.xteam(target, args)
	case "/open":
		return c.open(target)
	default:
		return Result{OK: false, Output: fmt.Sprintf("Unknown command %s. Type /help.", name)}
	}
}

func (c *Console) ask(target *project.Project, question string) Result {
	if strings.TrimSpace(question) == "" {
		return Result{OK: false, Output: "Usage: /ask <question>"}
	}
	job, err := c.App.Reviews.Enqueue(target.Slug, "ask", question, "console")
	if err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: fmt.Sprintf("Asked %s (read-only). The answer will appear here.", job.Reviewer), JobID: job.ID, Refresh: []string{"reviews"}}
}

func (c *Console) review(target *project.Project, args string) Result {
	kinds := strings.Fields(args)
	if len(kinds) == 0 {
		return Result{OK: false, Output: "Usage: /review <drift|code|security|deploy|test|all>"}
	}
	if len(kinds) == 1 && kinds[0] == "all" {
		kinds = review.ReviewKindIDs()
	}
	lines := []string{}
	jobIDs := []string{}
	for _, kind := range kinds {
		job, err := c.App.Reviews.Enqueue(target.Slug, kind, "", "console")
		switch {
		case errors.Is(err, review.ErrAlreadyActive):
			lines = append(lines, fmt.Sprintf("%s: already running (%s)", kind, job.ID))
		case err != nil:
			lines = append(lines, fmt.Sprintf("%s: %v", kind, err))
		default:
			jobIDs = append(jobIDs, job.ID)
			lines = append(lines, fmt.Sprintf("%s: queued with %s (%s)", kind, job.Reviewer, job.ID))
		}
	}
	return Result{OK: len(jobIDs) > 0, Output: strings.Join(lines, "\n"), JobIDs: jobIDs, Refresh: []string{"reviews", "brief"}}
}

func (c *Console) park(target *project.Project, args string) Result {
	if args == "" {
		return Result{OK: false, Output: "Usage: /park <where you stopped> | <next step> | <open questions>"}
	}
	parts := strings.Split(args, "|")
	entry := project.Entry{Stopped: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		entry.Next = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		entry.Open = strings.TrimSpace(strings.Join(parts[2:], "|"))
	}
	if _, err := c.App.Park(target.Slug, entry); err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: "Parked. Next time you open this project you will see this first.", Refresh: []string{"brief", "journal", "overview"}}
}

func (c *Console) note(target *project.Project, args string) Result {
	if _, err := c.App.Store.AppendJournal(target.Slug, project.Entry{Kind: project.EntryNote, Text: args}); err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: "Noted.", Refresh: []string{"journal"}}
}

func (c *Console) adr(target *project.Project, args string) Result {
	created, err := c.App.Store.CreateADR(target.Slug, args, "")
	if err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: fmt.Sprintf("Created %s. Fill in the decision in the Decisions tab.", created.ID), Refresh: []string{"adrs"}}
}

func (c *Console) status(target *project.Project, args string) Result {
	value := strings.ToLower(strings.TrimSpace(args))
	if _, err := c.App.Store.Update(target.Slug, project.Patch{Status: &value}); err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: "Status set to " + value + ".", Refresh: []string{"overview", "brief"}}
}

func (c *Console) scan(target *project.Project) Result {
	snapshot, err := c.App.ScanProject(target.Slug)
	if err != nil {
		return fail(err)
	}
	return Result{OK: true, Output: fmt.Sprintf("Scanned in %dms: %d docs, checklist %d/%d.", snapshot.DurationMs, len(snapshot.Docs), snapshot.Checklist.Done, snapshot.Checklist.Total), Refresh: []string{"brief", "docs", "overview"}}
}

func (c *Console) xteam(target *project.Project, args string) Result {
	if _, err := os.Stat(filepath.Join(target.Path, ".xteam")); err != nil {
		return Result{OK: false, Output: "This project has no .xteam directory; xteam is not set up here."}
	}
	if args == "" {
		return Result{OK: false, Output: "Usage: /xteam <message for pm>  or  /xteam status"}
	}
	argv := []string{"say", "pm", args}
	if args == "status" {
		argv = []string{"status"}
	}
	output, err := runExternal(target.Path, "xteam", argv...)
	if err != nil {
		return Result{OK: false, Output: strings.TrimSpace(output + "\n" + err.Error())}
	}
	if args != "status" {
		if _, err := c.App.Store.AppendJournal(target.Slug, project.Entry{Kind: project.EntryCommand, Text: "xteam say pm: " + args}); err != nil {
			c.App.Logger.Printf("console: journal xteam command: %v", err)
		}
	}
	return Result{OK: true, Output: nonEmpty(strings.TrimSpace(output), "Delivered to xteam pm.")}
}

func (c *Console) open(target *project.Project) Result {
	if _, err := review.LookupAgent("cursor"); err == nil {
		if output, err := runExternal(target.Path, "cursor", target.Path); err != nil {
			return Result{OK: false, Output: strings.TrimSpace(output + "\n" + err.Error())}
		}
		return Result{OK: true, Output: "Opened in Cursor."}
	}
	if output, err := runExternal(target.Path, "open", target.Path); err != nil {
		return Result{OK: false, Output: strings.TrimSpace(output + "\n" + err.Error())}
	}
	return Result{OK: true, Output: "Opened in Finder."}
}

// History returns the newest console lines first.
func (c *Console) History(slug string, limit int) ([]HistoryItem, error) {
	if _, err := c.App.Store.Get(slug); err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(c.App.Store.Dir(slug), "commands.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return []HistoryItem{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open command history: %w", err)
	}
	defer file.Close()
	items := []HistoryItem{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var item HistoryItem
		if json.Unmarshal(scanner.Bytes(), &item) == nil {
			items = append(items, item)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read command history: %w", err)
	}
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items, nil
}

func (c *Console) record(slug, input string, result Result) {
	line, err := json.Marshal(HistoryItem{At: time.Now().UTC(), Input: input, Result: result})
	if err != nil {
		c.App.Logger.Printf("console: encode history: %v", err)
		return
	}
	path := filepath.Join(c.App.Store.Dir(slug), "commands.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		c.App.Logger.Printf("console: open history: %v", err)
		return
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		c.App.Logger.Printf("console: write history: %v", err)
	}
}

// runExternal runs a helper binary (xteam, cursor, open) with a timeout.
func runExternal(dir, binary string, args ...string) (string, error) {
	resolved, err := review.LookupAgent(binary)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), externalTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, resolved, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+review.AugmentedPath(), "NO_COLOR=1")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	text := output.String()
	if len(text) > 20000 {
		text = text[:20000] + "\n…(truncated)"
	}
	return text, err
}

func helpText() string {
	var b strings.Builder
	b.WriteString("Plain text asks the reviewer agent (read-only). Commands:\n")
	for _, command := range Commands {
		fmt.Fprintf(&b, "  %-11s %-48s %s\n", command.Name, command.Args, command.Summary)
	}
	return b.String()
}

func fail(err error) Result { return Result{OK: false, Output: err.Error()} }

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
