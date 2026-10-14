package mcp

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/insight"
	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/review"
)

const (
	capADRDecision   = 600
	capFindingText   = 300
	capCommitTitle   = 120
	capReviewSummary = 400
)

func renderProjects(cards []insight.Card, status string) string {
	var b strings.Builder
	shown := 0
	for _, card := range cards {
		if status != "" && card.Status != status {
			continue
		}
		shown++
		fmt.Fprintf(&b, "- **%s** (project `%s`) · %s · health %s\n", card.Name, card.Slug, card.Status, card.HealthLevel)
		fmt.Fprintf(&b, "  Path: %s\n", card.Path)
		if card.Summary != "" {
			fmt.Fprintf(&b, "  %s\n", oneLine(card.Summary, capFindingText))
		}
		if verdicts := verdictList(card.Verdicts); verdicts != "" {
			fmt.Fprintf(&b, "  Latest verdicts: %s\n", verdicts)
		}
	}
	if shown == 0 {
		if status != "" {
			return fmt.Sprintf("No %s projects in xplan.", status)
		}
		return "No projects are registered in xplan yet."
	}
	return fmt.Sprintf("%d projects in xplan:\n\n%s", shown, b.String())
}

func renderContext(brief *insight.Brief, notes []project.Entry, findings []app.InboxItem) string {
	card := brief.Card
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: project context from xplan\n\n", card.Name)
	fmt.Fprintf(&b, "Path: %s · Status: %s · Health: %s\n", card.Path, card.Status, card.HealthLevel)
	if card.Branch != "" {
		fmt.Fprintf(&b, "Branch: %s", card.Branch)
		if card.LastCommitAt != nil {
			fmt.Fprintf(&b, " · Last commit %s: %s", formatTime(*card.LastCommitAt), oneLine(card.LastCommitSubject, capCommitTitle))
		}
		b.WriteString("\n")
	}
	if len(card.Stack) > 0 {
		fmt.Fprintf(&b, "Stack: %s\n", strings.Join(card.Stack, ", "))
	}
	if card.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", oneLine(card.Summary, capFindingText))
	}
	b.WriteString("\n> The intent below is the owner's and is read-only for agents. If your task conflicts with it, stop and ask the owner. Write a park note (xplan_park) before you stop.\n")

	writeIntent(&b, brief.Intent, card.IntentFilled, card.IntentDraft)
	writeADRs(&b, brief.ADRs)
	writePark(&b, card.LastPark)
	writeNotes(&b, notes)
	writeVerdicts(&b, brief.LatestDone, brief.Active)
	writeFindings(&b, findings)
	writeHealth(&b, card.Health)
	return b.String()
}

func writeIntent(b *strings.Builder, intent string, filled, draft bool) {
	b.WriteString("\n## Intent\n\n")
	switch {
	case !filled:
		b.WriteString("(Not written yet. Ask the owner why this project exists before making product decisions.)\n")
		return
	case draft:
		b.WriteString("(This is an agent's draft the owner has not confirmed yet; treat it as a hint, not a commitment.)\n\n")
	}
	b.WriteString(strings.TrimSpace(stripTitle(intent)))
	b.WriteString("\n")
}

func writeADRs(b *strings.Builder, adrs []project.ADR) {
	if len(adrs) == 0 {
		return
	}
	b.WriteString("\n## Decisions (ADRs)\n\n")
	for _, adr := range adrs {
		fmt.Fprintf(b, "- %s (%s)\n", adr.Title, adr.Status)
		if decision, found := project.SectionBody(adr.Content, "## Decision"); found && strings.TrimSpace(decision) != "" {
			fmt.Fprintf(b, "  Decision: %s\n", oneLine(decision, capADRDecision))
		}
	}
}

func writePark(b *strings.Builder, park *project.Entry) {
	if park == nil {
		return
	}
	fmt.Fprintf(b, "\n## Where work stopped (park note, %s)\n\n", formatTime(park.At))
	for _, field := range []struct{ label, value string }{
		{"Stopped", park.Stopped}, {"Next", park.Next}, {"Open", park.Open}, {"Note", park.Text},
	} {
		if strings.TrimSpace(field.value) != "" {
			fmt.Fprintf(b, "- %s: %s\n", field.label, strings.TrimSpace(field.value))
		}
	}
}

func writeNotes(b *strings.Builder, notes []project.Entry) {
	if len(notes) == 0 {
		return
	}
	b.WriteString("\n## Recent notes\n\n")
	for _, entry := range notes {
		fmt.Fprintf(b, "- %s: %s\n", formatTime(entry.At), strings.TrimSpace(entry.Text))
	}
}

func writeVerdicts(b *strings.Builder, latest map[string]*review.Job, active []*review.Job) {
	if len(latest) == 0 && len(active) == 0 {
		return
	}
	b.WriteString("\n## Reviews\n\n")
	kinds := make([]string, 0, len(latest))
	for kind := range latest {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		job := latest[kind]
		fmt.Fprintf(b, "- %s: %s", kind, job.Verdict)
		if job.Score > 0 {
			fmt.Fprintf(b, " (%d)", job.Score)
		}
		fmt.Fprintf(b, ", %s, id %s", formatTime(job.CreatedAt), job.ID)
		if job.Summary != "" {
			fmt.Fprintf(b, ": %s", oneLine(job.Summary, capReviewSummary))
		}
		b.WriteString("\n")
	}
	for _, job := range active {
		fmt.Fprintf(b, "- %s: %s now (id %s)\n", job.Kind, job.Status, job.ID)
	}
}

func writeFindings(b *strings.Builder, findings []app.InboxItem) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## Open findings (medium and above, %d)\n\n", len(findings))
	for index, item := range findings {
		if index == maxContextIssues {
			fmt.Fprintf(b, "- … %d more: call xplan_inbox\n", len(findings)-index)
			break
		}
		writeFinding(b, item, false)
	}
}

func writeHealth(b *strings.Builder, flags []insight.Flag) {
	if len(flags) == 0 {
		return
	}
	b.WriteString("\n## Health\n\n")
	for _, flag := range flags {
		fmt.Fprintf(b, "- %s: %s\n", flag.Level, flag.Message)
	}
}

func renderInbox(items []app.InboxItem, minimum string, allProjects bool) string {
	if len(items) == 0 {
		return fmt.Sprintf("No open findings at %s or above.", minimum)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d open findings at %s or above, most severe first:\n\n", len(items), minimum)
	for index, item := range items {
		if index == maxInboxItems {
			fmt.Fprintf(&b, "- … %d more; raise min_severity to narrow the list\n", len(items)-index)
			break
		}
		writeFinding(&b, item, allProjects)
	}
	return b.String()
}

func writeFinding(b *strings.Builder, item app.InboxItem, withProject bool) {
	finding := item.Finding
	fmt.Fprintf(b, "- [%s] ", finding.Severity)
	if withProject {
		fmt.Fprintf(b, "%s · ", item.ProjectName)
	}
	fmt.Fprintf(b, "%s: %s", item.Kind, finding.Title)
	if finding.File != "" {
		fmt.Fprintf(b, " (%s)", finding.File)
	}
	b.WriteString("\n")
	if finding.Detail != "" {
		fmt.Fprintf(b, "  %s\n", oneLine(finding.Detail, capFindingText))
	}
	if finding.Suggestion != "" {
		fmt.Fprintf(b, "  Suggestion: %s\n", oneLine(finding.Suggestion, capFindingText))
	}
}

func renderReview(projectName string, job *review.Job, report string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s review of %s (id %s)\n\n", job.Kind, projectName, job.ID)
	fmt.Fprintf(&b, "Status: %s · Reviewer: %s · Started %s\n", job.Status, job.Reviewer, formatTime(job.CreatedAt))
	if job.Question != "" {
		fmt.Fprintf(&b, "Question: %s\n", job.Question)
	}
	switch job.Status {
	case review.StatusQueued, review.StatusRunning:
		b.WriteString("\nStill running. Check again in a minute or two.\n")
		return b.String()
	case review.StatusFailed, review.StatusCanceled:
		fmt.Fprintf(&b, "\nNo report: %s\n", nonEmpty(job.Error, job.Status))
		return b.String()
	}
	if job.Verdict != "" {
		fmt.Fprintf(&b, "Verdict: %s", job.Verdict)
		if job.Score > 0 {
			fmt.Fprintf(&b, " · Score: %d", job.Score)
		}
		b.WriteString("\n")
	}
	if job.Summary != "" {
		fmt.Fprintf(&b, "Summary: %s\n", job.Summary)
	}
	if len(job.Findings) > 0 {
		b.WriteString("\n## Findings\n\n")
		for _, finding := range job.Findings {
			writeFinding(&b, app.InboxItem{Kind: job.Kind, Finding: finding}, false)
		}
	}
	if strings.TrimSpace(report) != "" {
		b.WriteString("\n## Report\n\n")
		b.WriteString(strings.TrimSpace(report))
		b.WriteString("\n")
	}
	return b.String()
}

func verdictList(verdicts map[string]string) string {
	kinds := make([]string, 0, len(verdicts))
	for kind := range verdicts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, kind+" "+verdicts[kind])
	}
	return strings.Join(parts, ", ")
}

// stripTitle drops the intent's "# Title" line; the context has its own.
func stripTitle(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "# ") {
		if newline := strings.Index(trimmed, "\n"); newline >= 0 {
			return trimmed[newline+1:]
		}
		return ""
	}
	return trimmed
}

func formatTime(at time.Time) string {
	return at.Local().Format("2006-01-02 15:04")
}

func oneLine(text string, limit int) string {
	return clipRunes(strings.Join(strings.Fields(text), " "), limit)
}

func clipRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
