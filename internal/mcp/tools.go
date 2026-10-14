package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/review"
)

// Tool is one MCP tool as listed to clients, plus its implementation.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations,omitempty"`
	run         func(context.Context, json.RawMessage) (string, error)
}

// ReadOnly reports whether the tool only reads xplan's records.
func (t Tool) ReadOnly() bool { return t.Annotations["readOnlyHint"] == true }

// TriggerMCP marks reviews started by an agent through MCP.
const TriggerMCP = "mcp"

// Size limits keep agent writes reviewable by a human.
const (
	maxNoteRunes     = 4000
	maxTitleRunes    = 120
	maxADRPartRunes  = 8000
	maxReportRunes   = 16000
	maxInboxItems    = 30
	maxContextNotes  = 5
	maxContextIssues = 12
)

var readOnly = map[string]any{"readOnlyHint": true, "openWorldHint": false}
var appendOnly = map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": false}

func (h *Handler) toolset() []Tool {
	reviewKinds := agentReviewKinds()
	return []Tool{
		{
			Name:        "xplan_projects",
			Title:       "List xplan projects",
			Description: "List the projects registered in xplan with their status, health, path and latest review verdicts.",
			InputSchema: schema(map[string]any{
				"status": enumOf("Only projects with this status.", project.Statuses),
			}),
			Annotations: readOnly,
			run:         typed(h.listProjects),
		},
		{
			Name:  "xplan_context",
			Title: "Project context",
			Description: "Read what the owner recorded about a project: intent (why it exists, goals, non-goals), decisions (ADRs), " +
				"where work last stopped, recent notes, latest review verdicts and open findings. Call this when a session starts.",
			InputSchema: schema(targetProperties),
			Annotations: readOnly,
			run:         typed(h.projectContext),
		},
		{
			Name:  "xplan_park",
			Title: "Write a park note",
			Description: "Record where work stopped so the next session (yours or the owner's) can pick it up. " +
				"Fill at least one of stopped, next and open.",
			InputSchema: schema(withTarget(map[string]any{
				"stopped": text("What was done and where you stopped."),
				"next":    text("The next concrete step."),
				"open":    text("Open questions or risks the owner should know about."),
			})),
			Annotations: appendOnly,
			run:         typed(h.park),
		},
		{
			Name:        "xplan_note",
			Title:       "Add a note",
			Description: "Add a short note to the project's journal for the owner to read later. Not for secrets or transcripts.",
			InputSchema: schema(withTarget(map[string]any{
				"text": text(fmt.Sprintf("The note, at most %d characters.", maxNoteRunes)),
			}), "text"),
			Annotations: appendOnly,
			run:         typed(h.note),
		},
		{
			Name:  "xplan_propose_adr",
			Title: "Propose a decision",
			Description: "Propose an architecture or product decision as an ADR with status \"proposed\". " +
				"The owner accepts or rejects it in xplan; you cannot accept it yourself.",
			InputSchema: schema(withTarget(map[string]any{
				"title":        text("Short decision title, e.g. \"Store sessions in Redis\"."),
				"context":      text("The situation and forces that call for a decision."),
				"decision":     text("What is proposed."),
				"consequences": text("What becomes easier or harder."),
			}), "title", "decision"),
			Annotations: appendOnly,
			run:         typed(h.proposeADR),
		},
		{
			Name:        "xplan_inbox",
			Title:       "Open review findings",
			Description: "List findings from the latest finished reviews, most severe first. Without path or project it covers every project.",
			InputSchema: schema(withTarget(map[string]any{
				"min_severity": enumOf("Lowest severity to include (default high).", []string{"critical", "high", "medium", "low", "info"}),
			})),
			Annotations: readOnly,
			run:         typed(h.inbox),
		},
		{
			Name:  "xplan_start_review",
			Title: "Start a review",
			Description: "Ask xplan's read-only reviewer agent for a second opinion. Kinds: drift (still on the intent?), code, security, deploy, test, " +
				"or ask (answer a question; needs question). Returns a review id; reviews take minutes.",
			InputSchema: schema(withTarget(map[string]any{
				"kind":     enumOf("Review kind.", reviewKinds),
				"question": text("The question, for kind ask."),
			}), "kind"),
			Annotations: appendOnly,
			run:         typed(h.startReview),
		},
		{
			Name:        "xplan_get_review",
			Title:       "Get a review",
			Description: "Read a review's status, verdict, findings and report. Without id it returns the most recent review (of kind, if given).",
			InputSchema: schema(withTarget(map[string]any{
				"id":   text("Review id from xplan_start_review or xplan_context."),
				"kind": enumOf("Most recent review of this kind.", reviewKinds),
			})),
			Annotations: readOnly,
			run:         typed(h.getReview),
		},
	}
}

// agentReviewKinds excludes intent drafts on purpose: they write intent.md,
// and the intent belongs to the owner.
func agentReviewKinds() []string {
	return append(review.ReviewKindIDs(), review.KindAsk)
}

type projectsInput struct {
	Status string `json:"status"`
}

func (h *Handler) listProjects(_ context.Context, input projectsInput) (string, error) {
	cards, err := h.app.Overview()
	if err != nil {
		return "", err
	}
	return renderProjects(cards, input.Status), nil
}

func (h *Handler) projectContext(_ context.Context, input Target) (string, error) {
	target, err := h.resolve(input)
	if err != nil {
		return "", err
	}
	brief, err := h.app.Brief(target.Slug)
	if err != nil {
		return "", err
	}
	notes, err := h.app.Store.ReadJournal(target.Slug, project.EntryNote, maxContextNotes)
	if err != nil {
		return "", err
	}
	findings, err := h.app.Inbox(target.Slug, "medium")
	if err != nil {
		return "", err
	}
	return renderContext(brief, notes, findings), nil
}

type parkInput struct {
	Target
	Stopped string `json:"stopped"`
	Next    string `json:"next"`
	Open    string `json:"open"`
}

func (h *Handler) park(_ context.Context, input parkInput) (string, error) {
	target, err := h.resolve(input.Target)
	if err != nil {
		return "", err
	}
	for _, field := range []string{input.Stopped, input.Next, input.Open} {
		if err := withinLimit(field, maxNoteRunes); err != nil {
			return "", err
		}
	}
	saved, err := h.app.Park(target.Slug, project.Entry{Stopped: strings.TrimSpace(input.Stopped), Next: strings.TrimSpace(input.Next), Open: strings.TrimSpace(input.Open)})
	if err != nil {
		return "", err
	}
	at := fmt.Sprintf("at %s", formatTime(saved.At))
	if saved.Head != "" {
		at += fmt.Sprintf(", commit %.7s", saved.Head)
	}
	return fmt.Sprintf("Park note saved for %s (%s). The owner sees it in xplan when they come back.", target.Name, at), nil
}

type noteInput struct {
	Target
	Text string `json:"text"`
}

func (h *Handler) note(_ context.Context, input noteInput) (string, error) {
	target, err := h.resolve(input.Target)
	if err != nil {
		return "", err
	}
	if err := withinLimit(input.Text, maxNoteRunes); err != nil {
		return "", err
	}
	if _, err := h.app.Store.AppendJournal(target.Slug, project.Entry{Kind: project.EntryNote, Text: strings.TrimSpace(input.Text)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Note saved to %s's journal.", target.Name), nil
}

type adrInput struct {
	Target
	Title        string `json:"title"`
	Context      string `json:"context"`
	Decision     string `json:"decision"`
	Consequences string `json:"consequences"`
}

func (h *Handler) proposeADR(_ context.Context, input adrInput) (string, error) {
	target, err := h.resolve(input.Target)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(input.Decision) == "" {
		return "", errors.New("decision is required: say what you propose")
	}
	if err := withinLimit(input.Title, maxTitleRunes); err != nil {
		return "", err
	}
	for _, part := range []string{input.Context, input.Decision, input.Consequences} {
		if err := withinLimit(part, maxADRPartRunes); err != nil {
			return "", err
		}
	}
	created, err := h.app.Store.CreateADR(target.Slug, input.Title, input.Context)
	if err != nil {
		return "", err
	}
	content := fillProposal(created.Content, strings.TrimSpace(input.Decision), strings.TrimSpace(input.Consequences))
	saved, err := h.app.Store.UpdateADR(target.Slug, created.ID, content)
	if err != nil {
		return "", fmt.Errorf("ADR %s was created but its decision could not be saved: %w", created.ID, err)
	}
	return fmt.Sprintf("Proposed ADR %04d \"%s\" for %s (status: %s). The owner accepts or rejects it in xplan.", saved.Number, input.Title, target.Name, saved.Status), nil
}

// fillProposal writes the decision into the ADR template and marks who
// proposed it. The status line above stays "proposed".
func fillProposal(content, decision, consequences string) string {
	content = strings.Replace(content, "\n\n## Context", "\n- Proposed by: an agent (xplan MCP)\n\n## Context", 1)
	content = strings.Replace(content, "## Decision\n\n", "## Decision\n\n"+decision+"\n", 1)
	if consequences != "" {
		content = strings.Replace(content, "## Consequences\n\n", "## Consequences\n\n"+consequences+"\n", 1)
	}
	return content
}

type inboxInput struct {
	Target
	MinSeverity string `json:"min_severity"`
}

func (h *Handler) inbox(_ context.Context, input inboxInput) (string, error) {
	minimum := strings.TrimSpace(input.MinSeverity)
	if minimum == "" {
		minimum = "high"
	}
	if _, ok := app.SeverityRank[minimum]; !ok {
		return "", fmt.Errorf("unknown severity %q", minimum)
	}
	slug := ""
	if input.Project != "" || input.Path != "" {
		target, err := h.resolve(input.Target)
		if err != nil {
			return "", err
		}
		slug = target.Slug
	}
	items, err := h.app.Inbox(slug, minimum)
	if err != nil {
		return "", err
	}
	return renderInbox(items, minimum, slug == ""), nil
}

type startReviewInput struct {
	Target
	Kind     string `json:"kind"`
	Question string `json:"question"`
}

func (h *Handler) startReview(_ context.Context, input startReviewInput) (string, error) {
	target, err := h.resolve(input.Target)
	if err != nil {
		return "", err
	}
	if !slices.Contains(agentReviewKinds(), input.Kind) {
		return "", fmt.Errorf("kind must be one of %s", strings.Join(agentReviewKinds(), ", "))
	}
	job, err := h.app.Reviews.Enqueue(target.Slug, input.Kind, input.Question, TriggerMCP)
	if errors.Is(err, review.ErrAlreadyActive) {
		return fmt.Sprintf("A %s review of %s is already running (id %s). Check it with xplan_get_review.", input.Kind, target.Name, job.ID), nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Started a %s review of %s (id %s, reviewer %s). Reviews take a few minutes; read it with xplan_get_review.", job.Kind, target.Name, job.ID, job.Reviewer), nil
}

type getReviewInput struct {
	Target
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func (h *Handler) getReview(_ context.Context, input getReviewInput) (string, error) {
	target, err := h.resolve(input.Target)
	if err != nil {
		return "", err
	}
	job, err := h.findReview(target.Slug, strings.TrimSpace(input.ID), strings.TrimSpace(input.Kind))
	if err != nil {
		return "", err
	}
	report := ""
	if job.Status == review.StatusDone {
		if report, err = h.app.Reviews.Output(target.Slug, job.ID); err != nil {
			return "", err
		}
	}
	if utf8.RuneCountInString(report) > maxReportRunes {
		report = clipRunes(report, maxReportRunes) + "\n\n(Report clipped; the full text is in xplan.)"
	}
	return renderReview(target.Name, job, report), nil
}

func (h *Handler) findReview(slug, id, kind string) (*review.Job, error) {
	if id != "" {
		return h.app.Reviews.Get(slug, id)
	}
	jobs, err := h.app.Reviews.List(slug, 100)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job.Kind == review.KindIntent {
			continue
		}
		if kind == "" || job.Kind == kind {
			return job, nil
		}
	}
	if kind != "" {
		return nil, fmt.Errorf("no %s review yet; start one with xplan_start_review", kind)
	}
	return nil, errors.New("no reviews yet; start one with xplan_start_review")
}

// typed decodes tool arguments into the tool's input struct.
func typed[T any](fn func(context.Context, T) (string, error)) func(context.Context, json.RawMessage) (string, error) {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var input T
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", fmt.Errorf("invalid arguments: %v", err)
			}
		}
		return fn(ctx, input)
	}
}

func schema(properties map[string]any, required ...string) map[string]any {
	result := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		result["required"] = required
	}
	return result
}

func withTarget(properties map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range targetProperties {
		merged[key] = value
	}
	for key, value := range properties {
		merged[key] = value
	}
	return merged
}

func text(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func enumOf(description string, values []string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

func withinLimit(value string, limit int) error {
	if count := utf8.RuneCountInString(strings.TrimSpace(value)); count > limit {
		return fmt.Errorf("text is %d characters; keep it under %d and point to a file in the repository for more", count, limit)
	}
	return nil
}
