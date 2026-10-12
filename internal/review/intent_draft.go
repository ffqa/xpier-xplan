package review

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/project"
)

// ErrNoIntentDocument means the agent answered without the expected sections.
var ErrNoIntentDocument = errors.New(`the agent's answer has no "## Why" section, so no draft was written`)

var (
	markdownFence = regexp.MustCompile("(?s)```(?:markdown|md)?[ \\t]*\\n(.*?)\\n```")
	whyHeading    = regexp.MustCompile(`(?mi)^##\s+why\b`)
)

// ExtractIntentDraft pulls the intent document out of an agent's answer
// (dropping any preamble, code fence or JSON block), restores the canonical
// headings, and stamps it with the draft marker.
func ExtractIntentDraft(output, projectName, reviewer string, at time.Time) (string, error) {
	_, body, _ := ParseReport(output)
	body = unwrapIntentFence(body)
	start := whyHeading.FindStringIndex(body)
	if start == nil {
		return "", ErrNoIntentDocument
	}
	sections := canonicalHeadings(strings.TrimSpace(body[start[0]:]))

	var builder strings.Builder
	builder.WriteString(project.IntentTitle(projectName) + "\n\n")
	fmt.Fprintf(&builder, "%s — inferred by %s from the README, docs and git history on %s. Correct it in your own words, then delete this line.\n\n",
		project.DraftMarker, reviewer, at.Format("2006-01-02"))
	builder.WriteString(sections + "\n")
	return builder.String(), nil
}

// unwrapIntentFence returns the content of a fenced block that holds the
// document, for agents that wrap their whole answer in ```markdown.
func unwrapIntentFence(body string) string {
	for _, match := range markdownFence.FindAllStringSubmatch(body, -1) {
		if whyHeading.MatchString(match[1]) {
			return match[1]
		}
	}
	return body
}

// canonicalHeadings rewrites "## Goal" or "## Why — problem" to the exact
// headings IntentFilled and the brief look for. Unknown headings stay as-is.
func canonicalHeadings(sections string) string {
	lines := strings.Split(sections, "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		if heading, ok := matchSection(line); ok {
			lines[index] = heading
		}
	}
	return strings.Join(lines, "\n")
}

func matchSection(line string) (string, bool) {
	word := firstWord(line)
	if word == "" {
		return "", false
	}
	for _, heading := range project.IntentSections {
		if firstWord(heading) == word {
			return heading, true
		}
	}
	return "", false
}

// firstWord returns the lower-cased first word of a "## " heading.
func firstWord(heading string) string {
	fields := strings.Fields(strings.TrimPrefix(heading, "## "))
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimRight(strings.ToLower(fields[0]), ":")
}
