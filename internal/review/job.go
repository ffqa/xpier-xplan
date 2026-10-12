package review

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// Job statuses.
const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusDone     = "done"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// Verdicts produced by review kinds.
const (
	VerdictOK        = "ok"
	VerdictAttention = "attention"
	VerdictCritical  = "critical"
	VerdictUnknown   = "unknown"
)

// Finding is one issue reported by a reviewer.
type Finding struct {
	Severity   string `json:"severity"`
	Title      string `json:"title"`
	File       string `json:"file,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Job is one review run, persisted as reviews/<id>.json.
type Job struct {
	ID          string     `json:"id"`
	Project     string     `json:"project"`
	Kind        string     `json:"kind"`
	Question    string     `json:"question,omitempty"`
	Reviewer    string     `json:"reviewer"`
	Trigger     string     `json:"trigger"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	ExitCode    int        `json:"exit_code"`
	Error       string     `json:"error,omitempty"`
	Verdict     string     `json:"verdict,omitempty"`
	Score       int        `json:"score,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Findings    []Finding  `json:"findings"`
	TreeChanged bool       `json:"tree_changed"`
	PromptChars int        `json:"prompt_chars"`
	// IntentWritten is set by intent drafts that filled an empty intent.md;
	// false means the owner's intent was kept and the draft is the report.
	IntentWritten bool `json:"intent_written,omitempty"`
	// ReviewerRequested is "auto" when Reviewer was picked at run time.
	ReviewerRequested string `json:"reviewer_requested,omitempty"`
	// Attempts lists reviewers "auto" skipped, as "name: reason".
	Attempts []string `json:"attempts,omitempty"`
}

// Active reports whether the job is still queued or running.
func (j *Job) Active() bool {
	return j.Status == StatusQueued || j.Status == StatusRunning
}

// SeverityCounts tallies findings by severity.
func (j *Job) SeverityCounts() map[string]int {
	counts := map[string]int{}
	for _, finding := range j.Findings {
		counts[finding.Severity]++
	}
	return counts
}

var jsonFence = regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n?```")

type parsedReport struct {
	Verdict  string    `json:"verdict"`
	Score    int       `json:"score"`
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}

// ParseReport extracts the trailing JSON block from an agent report. It
// returns the parsed data, the Markdown with that block removed, and whether
// parsing succeeded.
func ParseReport(output string) (parsedReport, string, bool) {
	matches := jsonFence.FindAllStringSubmatchIndex(output, -1)
	for index := len(matches) - 1; index >= 0; index-- {
		match := matches[index]
		body := output[match[2]:match[3]]
		var report parsedReport
		if err := json.Unmarshal([]byte(body), &report); err != nil {
			continue
		}
		report.Verdict = normalizeVerdict(report.Verdict)
		for findingIndex := range report.Findings {
			report.Findings[findingIndex].Severity = normalizeSeverity(report.Findings[findingIndex].Severity)
		}
		if report.Findings == nil {
			report.Findings = []Finding{}
		}
		cleaned := strings.TrimSpace(output[:match[0]] + output[match[1]:])
		return report, cleaned, true
	}
	return parsedReport{Verdict: VerdictUnknown, Findings: []Finding{}}, strings.TrimSpace(output), false
}

func normalizeVerdict(verdict string) string {
	switch strings.ToLower(strings.TrimSpace(verdict)) {
	case "ok", "aligned", "pass", "good":
		return VerdictOK
	case "attention", "drifting", "warn", "warning":
		return VerdictAttention
	case "critical", "off-track", "fail", "bad":
		return VerdictCritical
	default:
		return VerdictUnknown
	}
}

func normalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "blocker":
		return "critical"
	case "high", "major":
		return "high"
	case "medium", "moderate":
		return "medium"
	case "low", "minor":
		return "low"
	default:
		return "info"
	}
}
