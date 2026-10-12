package review

import (
	"fmt"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/scanner"
)

// PromptInput is everything the context bundle is built from.
type PromptInput struct {
	Project      *project.Project
	Kind         Kind
	Question     string
	Intent       string
	ADRs         []project.ADR
	LastPark     *project.Entry
	Snapshot     *scanner.Snapshot
	PreviousJob  *Job
	Language     string
	ProjectNotes string
	FirstCommits []scanner.Commit
}

// Size caps keep the prompt well below argv limits and model context.
const (
	capIntent       = 6000
	capADR          = 500
	capADRTotal     = 3500
	capChangelog    = 1500
	maxDocLines     = 80
	maxCommits      = 20
	maxSlices       = 15
	maxFirstCommits = 15
)

// BuildPrompt renders the full prompt handed to the agent.
func BuildPrompt(input PromptInput) string {
	var b strings.Builder
	name := input.Project.Name
	fmt.Fprintf(&b, "You are an external reviewer for the software project %q, located in the current working directory.\n", name)
	b.WriteString("You have NOT taken part in building it, which is why your view is valuable.\n")
	b.WriteString("STRICTLY READ-ONLY: do not create, modify or delete any file, do not install anything, and do not run commands that change state.\n")
	b.WriteString("Use the repository itself plus the context collected below.\n\n")

	b.WriteString("## Context collected by xplan\n\n")
	writeIntent(&b, input.Intent)
	writeADRs(&b, input.ADRs)
	writePark(&b, input.LastPark)
	writeSnapshot(&b, input.Snapshot)
	writeFirstCommits(&b, input.FirstCommits)
	writePrevious(&b, input.PreviousJob)

	fmt.Fprintf(&b, "## Your task: %s\n\n%s\n\n", input.Kind.Title, input.Kind.Instructions)
	switch input.Kind.ID {
	case KindAsk:
		fmt.Fprintf(&b, "Question from the owner:\n\n%s\n\n", strings.TrimSpace(input.Question))
		fmt.Fprintf(&b, "## Output format\n\nAnswer in %s, in concise Markdown.\n", input.Language)
		return b.String()
	case KindIntent:
		writeIntentFormat(&b, input.Language)
		return b.String()
	}

	fmt.Fprintf(&b, "## Output format\n\nWrite a concise Markdown report in %s with the sections: Summary, Findings, Recommendations.\n", input.Language)
	b.WriteString("Then END your answer with exactly one fenced JSON block (keys in English, values in the report language):\n\n")
	b.WriteString("```json\n")
	b.WriteString(`{"verdict": "ok | attention | critical", "score": 0, "summary": "one sentence", "findings": [{"severity": "critical | high | medium | low | info", "title": "", "file": "path:line or empty", "detail": "", "suggestion": ""}]}`)
	b.WriteString("\n```\n\nscore is 0-100 where 100 means no concerns at all.\n")
	return b.String()
}

// writeIntentFormat asks for the intent.md skeleton; ExtractIntentDraft relies
// on the English headings.
func writeIntentFormat(b *strings.Builder, language string) {
	fmt.Fprintf(b, "## Output format\n\nOutput ONLY the document below, filled in, with the content in %s. Keep the headings exactly as written, in English. No preamble, no closing remarks, no code fence.\n\n", language)
	last := len(project.IntentSections) - 1
	for index, heading := range project.IntentSections {
		b.WriteString(heading + "\n")
		if index == last {
			b.WriteString("(Quote the earliest description you can find, such as the first README text, early commit messages or an early spec, with its source in parentheses. Leave empty if nothing exists.)\n\n")
			continue
		}
		b.WriteString("- …\n\n")
	}
	b.WriteString("## Sources\n- path or commit — what it told you\n")
}

func writeIntent(b *strings.Builder, intent string) {
	if project.IntentIsDraft(intent) {
		b.WriteString("### Intent (a DRAFT inferred by an agent and not yet confirmed by the owner; treat it as low-confidence)\n\n")
		b.WriteString(truncate(strings.TrimSpace(intent), capIntent) + "\n\n")
		return
	}
	b.WriteString("### Intent (written by the owner; the source of truth for why this project exists)\n\n")
	if !project.IntentFilled(intent) {
		b.WriteString("(The owner has not written the intent yet.)\n\n")
		if excerpt := strings.TrimSpace(intent); excerpt != "" {
			b.WriteString(truncate(excerpt, capIntent) + "\n\n")
		}
		return
	}
	b.WriteString(truncate(strings.TrimSpace(intent), capIntent) + "\n\n")
}

func writeADRs(b *strings.Builder, adrs []project.ADR) {
	if len(adrs) == 0 {
		return
	}
	b.WriteString("### Decisions (ADRs kept by the owner)\n\n")
	total := 0
	for _, adr := range adrs {
		chunk := fmt.Sprintf("- [%s] %s\n%s\n", adr.Status, adr.Title, indent(truncate(adr.Content, capADR)))
		if total+len(chunk) > capADRTotal {
			b.WriteString("- …more ADRs omitted\n")
			break
		}
		total += len(chunk)
		b.WriteString(chunk)
	}
	b.WriteString("\n")
}

func writePark(b *strings.Builder, park *project.Entry) {
	if park == nil {
		return
	}
	fmt.Fprintf(b, "### Where the owner left off (%s)\n\n", park.At.Format("2006-01-02 15:04"))
	for _, field := range [][2]string{{"Stopped at", park.Stopped}, {"Next step", park.Next}, {"Open questions", park.Open}, {"Note", park.Text}} {
		if strings.TrimSpace(field[1]) != "" {
			fmt.Fprintf(b, "- %s: %s\n", field[0], strings.TrimSpace(field[1]))
		}
	}
	b.WriteString("\n")
}

func writeSnapshot(b *strings.Builder, snapshot *scanner.Snapshot) {
	if snapshot == nil {
		return
	}
	if len(snapshot.Stack) > 0 {
		fmt.Fprintf(b, "### Stack\n\n%s\n\n", strings.Join(snapshot.Stack, ", "))
	}
	if git := snapshot.Git; git != nil {
		fmt.Fprintf(b, "### Git\n\nBranch %s, %d commits in the last 7 days, %d in 30 days, %d uncommitted files.\n\n", git.Branch, git.Commits7d, git.Commits30d, git.DirtyFiles)
		if len(git.Recent) > 0 {
			b.WriteString("Recent commits:\n")
			for index, commit := range git.Recent {
				if index >= maxCommits {
					break
				}
				fmt.Fprintf(b, "- %s %s %s\n", commit.Short, commit.At.Format("2006-01-02"), commit.Subject)
			}
			b.WriteString("\n")
		}
	}
	if len(snapshot.Docs) > 0 {
		b.WriteString("### Documents the project keeps\n\n")
		for index, doc := range snapshot.Docs {
			if index >= maxDocLines {
				fmt.Fprintf(b, "- …and %d more\n", len(snapshot.Docs)-maxDocLines)
				break
			}
			fmt.Fprintf(b, "- [%s] %s — %s\n", doc.Kind, doc.Path, doc.Title)
		}
		b.WriteString("\n")
	}
	if snapshot.Checklist.Total > 0 {
		fmt.Fprintf(b, "Checklists across docs: %d/%d done.\n\n", snapshot.Checklist.Done, snapshot.Checklist.Total)
	}
	if changelog := snapshot.Changelog; changelog != nil && changelog.Version != "" {
		fmt.Fprintf(b, "### Latest changelog entry (%s %s)\n\n%s\n\n", changelog.Version, changelog.Date, truncate(changelog.Excerpt, capChangelog))
	}
	if xteam := snapshot.Xteam; xteam != nil {
		fmt.Fprintf(b, "### xteam slices (%d open, %d closed)\n\n", xteam.Open, xteam.Closed)
		for index, task := range xteam.Tasks {
			if index >= maxSlices {
				break
			}
			fmt.Fprintf(b, "- %s [%s%s] %s\n", task.Slug, task.Stage, verdictSuffix(task.Verdict), task.Title)
		}
		b.WriteString("\n")
	}
}

func writeFirstCommits(b *strings.Builder, commits []scanner.Commit) {
	if len(commits) == 0 {
		return
	}
	b.WriteString("### Earliest commits (oldest first)\n\n")
	for _, commit := range commits {
		fmt.Fprintf(b, "- %s %s %s\n", commit.Short, commit.At.Format("2006-01-02"), commit.Subject)
	}
	b.WriteString("\n")
}

func writePrevious(b *strings.Builder, previous *Job) {
	if previous == nil || previous.Summary == "" {
		return
	}
	fmt.Fprintf(b, "### Previous review of this kind (%s, verdict %s)\n\n%s\n\nSay whether things improved or regressed since then.\n\n",
		previous.CreatedAt.Format("2006-01-02"), previous.Verdict, previous.Summary)
}

func verdictSuffix(verdict string) string {
	if verdict == "" {
		return ""
	}
	return " " + verdict
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "\n…(truncated)"
}

func indent(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index, line := range lines {
		lines[index] = "  " + line
	}
	return strings.Join(lines, "\n")
}
