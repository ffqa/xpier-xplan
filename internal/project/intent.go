package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/config"
)

// Intent section headings; IntentFilled checks the first two.
const (
	headingWhy  = "## Why (the problem)"
	headingGoal = "## Goal (north star)"
)

// IntentSections are the headings of intent.md, in order.
var IntentSections = []string{
	headingWhy,
	headingGoal,
	"## Success looks like",
	"## Non-goals",
	"## Constraints",
	"## Original idea (verbatim)",
}

// DraftMarker starts the line an agent-written intent carries until the owner
// has corrected it and deleted that line.
const DraftMarker = "> **Draft**"

// IntentTitle is the first line of intent.md.
func IntentTitle(name string) string {
	return "# Intent — " + name
}

// IntentTemplate builds the initial intent.md for a newly registered project.
func IntentTemplate(name, readmeExcerpt string) string {
	var builder strings.Builder
	builder.WriteString(IntentTitle(name) + "\n\n")
	builder.WriteString("> Why this project exists. Owned by xplan, not by the project.\n")
	builder.WriteString("> Re-read this before touching the code again.\n\n")
	for _, heading := range IntentSections {
		builder.WriteString(heading + "\n\n\n")
	}
	if readmeExcerpt != "" {
		builder.WriteString("---\n\n")
		builder.WriteString("_Imported from the project's README at registration. Verify it, then rewrite the sections above in your own words._\n\n")
		for _, line := range strings.Split(readmeExcerpt, "\n") {
			builder.WriteString("> " + line + "\n")
		}
	}
	return builder.String()
}

// ReadIntent returns intent.md for a project (empty string if missing).
func (s *Store) ReadIntent(slug string) (string, error) {
	if _, err := s.Get(slug); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir(slug), "intent.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read intent %s: %w", slug, err)
	}
	return string(raw), nil
}

// WriteIntent replaces intent.md.
func (s *Store) WriteIntent(slug, content string) error {
	return config.WriteFileAtomic(filepath.Join(s.Dir(slug), "intent.md"), []byte(content))
}

// IntentFilled reports whether the user has written at least the Why or Goal
// section (anything outside comments and blank lines).
func IntentFilled(content string) bool {
	return sectionHasText(content, headingWhy) || sectionHasText(content, headingGoal)
}

// IntentIsDraft reports whether intent.md still carries the draft marker.
func IntentIsDraft(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), DraftMarker) {
			return true
		}
	}
	return false
}

func sectionHasText(content, heading string) bool {
	start := strings.Index(content, heading)
	if start < 0 {
		// The user restructured the file; treat any substantial content as filled.
		return len(strings.TrimSpace(content)) > 200
	}
	body := content[start+len(heading):]
	if next := strings.Index(body, "\n## "); next >= 0 {
		body = body[:next]
	}
	if rule := strings.Index(body, "\n---"); rule >= 0 {
		body = body[:rule]
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<!--") {
			return true
		}
	}
	return false
}

// readmeCandidates are tried in order when extracting a project description.
var readmeCandidates = []string{"README.md", "README.zh-CN.md", "readme.md", "PROJECT.md", "README"}

// ReadmeExcerpt returns the first descriptive paragraphs of the project's
// README (headings, badges, code fences and HTML skipped), capped at ~600 chars.
func ReadmeExcerpt(projectPath string) string {
	for _, candidate := range readmeCandidates {
		raw, err := os.ReadFile(filepath.Join(projectPath, candidate))
		if err != nil {
			continue
		}
		if excerpt := extractParagraphs(string(raw), 600); excerpt != "" {
			return excerpt
		}
	}
	return ""
}

func extractParagraphs(markdown string, limit int) string {
	var collected []string
	total := 0
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if strings.HasPrefix(trimmed, "## ") && len(collected) > 0 {
			break // only the opening section describes the project
		}
		if inFence || skipReadmeLine(trimmed) {
			if trimmed == "" && len(collected) > 0 && collected[len(collected)-1] != "" {
				collected = append(collected, "")
			}
			continue
		}
		collected = append(collected, trimmed)
		total += len(trimmed)
		if total >= limit {
			break
		}
	}
	return strings.TrimSpace(strings.Join(collected, "\n"))
}

func skipReadmeLine(trimmed string) bool {
	return trimmed == "" ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "![") ||
		strings.HasPrefix(trimmed, "[![") ||
		strings.HasPrefix(trimmed, "<") ||
		strings.HasPrefix(trimmed, "|") ||
		strings.HasPrefix(trimmed, "---") ||
		strings.HasPrefix(trimmed, "←") ||
		isNavigationLine(trimmed)
}

var (
	markdownLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
	htmlTag       = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	emphasisMarks = regexp.MustCompile("\\*\\*|__|`")
)

// isNavigationLine spots link bars such as "中文 | [English](README.en.md)"
// or "[Docs](docs) · [Demo](https://...)": separators plus links, little prose.
func isNavigationLine(line string) bool {
	if !markdownLink.MatchString(line) || !strings.ContainsAny(line, "|·•") {
		return false
	}
	prose := markdownLink.ReplaceAllString(line, "")
	prose = strings.Trim(prose, " |·•/-")
	return len([]rune(prose)) <= 12
}

// plainText strips the Markdown/HTML decoration a one-line summary should not show.
func plainText(text string) string {
	text = markdownLink.ReplaceAllString(text, "$1")
	text = htmlTag.ReplaceAllString(text, "")
	text = emphasisMarks.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(text), " ")
}

// firstSentence trims text to its first line within limit runes, as plain text.
func firstSentence(text string, limit int) string {
	text = strings.TrimSpace(text)
	if newline := strings.Index(text, "\n"); newline >= 0 {
		text = text[:newline]
	}
	text = plainText(strings.TrimLeft(text, "> "))
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return text
}
