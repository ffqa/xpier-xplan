package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/i18n"
)

// Reviewer health statuses.
const (
	HealthOK      = "ok"
	HealthAuth    = "auth"    // needs a login, an API key or a paid plan
	HealthMissing = "missing" // the binary is not on PATH
	HealthTimeout = "timeout"
	HealthError   = "error"
)

// How long a recorded result is trusted before "auto" probes again.
const (
	healthyFor   = 24 * time.Hour
	unhealthyFor = 6 * time.Hour
)

// ReviewerHealth is the latest known state of one reviewer preset.
type ReviewerHealth struct {
	Status     string    `json:"status"`
	Detail     string    `json:"detail,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	// Source is "test" (smoke test) or "run" (a real review).
	Source string `json:"source"`
}

// Fresh reports whether the result is recent enough to act on.
func (h ReviewerHealth) Fresh(now time.Time) bool {
	window := unhealthyFor
	if h.Status == HealthOK {
		window = healthyFor
	}
	return now.Sub(h.CheckedAt) < window
}

// HealthBook persists reviewer health in <home>/reviewers-health.json.
type HealthBook struct {
	path    string
	mu      sync.Mutex
	entries map[string]ReviewerHealth
}

// HealthFileName is the health book's file name inside the xplan home.
const HealthFileName = "reviewers-health.json"

// LoadHealthBook reads the book; a missing or corrupt file starts empty.
func LoadHealthBook(home string) (*HealthBook, error) {
	book := &HealthBook{path: filepath.Join(home, HealthFileName), entries: map[string]ReviewerHealth{}}
	raw, err := os.ReadFile(book.path)
	if errors.Is(err, os.ErrNotExist) {
		return book, nil
	}
	if err != nil {
		return book, fmt.Errorf("read %s: %w", book.path, err)
	}
	if err := json.Unmarshal(raw, &book.entries); err != nil {
		book.entries = map[string]ReviewerHealth{}
		return book, fmt.Errorf("parse %s: %w", book.path, err)
	}
	return book, nil
}

// Get returns the recorded health of a reviewer.
func (b *HealthBook) Get(name string) (ReviewerHealth, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	health, ok := b.entries[name]
	return health, ok
}

// All returns a copy of every entry.
func (b *HealthBook) All() map[string]ReviewerHealth {
	b.mu.Lock()
	defer b.mu.Unlock()
	copied := make(map[string]ReviewerHealth, len(b.entries))
	for name, health := range b.entries {
		copied[name] = health
	}
	return copied
}

// Record stores a result and persists the book.
func (b *HealthBook) Record(name string, health ReviewerHealth) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries[name] = health
	raw, err := json.MarshalIndent(b.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode reviewer health: %w", err)
	}
	return config.WriteFileAtomic(b.path, raw)
}

// authPattern matches what agent CLIs print when they cannot work without a
// login, an API key, credits or a paid plan.
var authPattern = regexp.MustCompile(`(?i)(authenticat|not logged in|\blog ?in first|please (run .*)?log ?in|unauthori[sz]ed|\b401\b|\b403\b|api[ _-]?key|credential|subscription|quota|credit balance|billing|\bexpired\b|opening browser|\bsign[ -]?in\b)`)

// ansiEscape matches terminal color and cursor codes that CLIs print even
// when their output is not a terminal.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// classifyFailure turns a failed run into a health status and a short detail.
func classifyFailure(result runResult, logText string) (string, string) {
	switch {
	case errors.Is(result.Err, ErrAgentMissing):
		return HealthMissing, result.Err.Error()
	case errors.Is(result.Err, errTimedOut):
		return HealthTimeout, result.Err.Error()
	}
	combined := result.Output + "\n" + logText
	if line := firstMatchingLine(combined, authPattern); line != "" {
		return HealthAuth, line
	}
	detail := lastLine(logText)
	if detail == "" && result.Err != nil {
		detail = result.Err.Error()
	}
	return HealthError, detail
}

// HealthLabel is the short human label for a status.
func HealthLabel(status string) string {
	switch status {
	case HealthOK:
		return i18n.T("working")
	case HealthAuth:
		return i18n.T("needs login or subscription")
	case HealthMissing:
		return i18n.T("not installed")
	case HealthTimeout:
		return i18n.T("timed out")
	default:
		return i18n.T("failed")
	}
}

func firstMatchingLine(text string, pattern *regexp.Regexp) string {
	text = ansiEscape.ReplaceAllString(text, "")
	for _, line := range strings.Split(text, "\n") {
		if pattern.MatchString(line) {
			return clip(strings.TrimSpace(line), 240)
		}
	}
	return ""
}

func lastLine(text string) string {
	text = ansiEscape.ReplaceAllString(text, "")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		if line := strings.TrimSpace(lines[index]); line != "" {
			return clip(line, 240)
		}
	}
	return ""
}

func clip(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
