// Package insight turns raw facts (project config, scan snapshot, journal,
// reviews) into the views the UI needs: health flags, board cards and the
// re-entry brief.
package insight

import (
	"fmt"
	"strings"
	"time"

	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/review"
	"github.com/ffqa/xplan/internal/scanner"
)

// Flag levels, ordered by severity.
const (
	LevelOK   = "ok"
	LevelInfo = "info"
	LevelWarn = "warn"
	LevelCrit = "crit"
)

// Flag is one health signal shown on cards and the brief.
type Flag struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Facts bundles everything needed to evaluate a project.
type Facts struct {
	Project      *project.Project
	Snapshot     *scanner.Snapshot
	State        project.State
	IntentFilled bool
	LastPark     *project.Entry
	LatestDone   map[string]*review.Job
	LastAttempts map[string]*review.Job
	ActiveJobs   int
	Now          time.Time
}

const staleAfter = 14 * 24 * time.Hour

// Health evaluates all flags for a project, most severe first.
func Health(facts Facts) []Flag {
	flags := []Flag{}
	add := func(level, code, message string) {
		flags = append(flags, Flag{Level: level, Code: code, Message: message})
	}
	snapshot := facts.Snapshot
	switch {
	case snapshot == nil:
		add(LevelInfo, "never_scanned", "Not scanned yet")
	case !snapshot.PathExists:
		add(LevelCrit, "path_missing", "Project directory is missing")
	}
	if !facts.IntentFilled {
		add(LevelWarn, "no_intent", "Intent not written yet — why does this project exist?")
	}
	if snapshot != nil && snapshot.PathExists {
		if snapshot.Git == nil {
			add(LevelWarn, "no_git", "Not a git repository")
		} else {
			git := snapshot.Git
			if facts.Project.Status == "active" && git.LastCommitAt != nil && facts.Now.Sub(*git.LastCommitAt) > staleAfter {
				add(LevelWarn, "stale", fmt.Sprintf("Marked active but no commit for %d days", int(facts.Now.Sub(*git.LastCommitAt).Hours()/24)))
			}
			if git.DirtyFiles > 30 {
				add(LevelWarn, "dirty", fmt.Sprintf("%d uncommitted files", git.DirtyFiles))
			} else if git.DirtyFiles > 0 {
				add(LevelInfo, "dirty", fmt.Sprintf("%d uncommitted files", git.DirtyFiles))
			}
		}
		if snapshot.Xteam != nil && snapshot.Xteam.Open > 0 {
			add(LevelInfo, "xteam_open", fmt.Sprintf("%d xteam slices in flight", snapshot.Xteam.Open))
		}
	}
	if facts.LastPark == nil && facts.Project.Status != "done" {
		add(LevelInfo, "no_park", "No park note — write where you stopped before switching away")
	}
	for _, kind := range review.ReviewKindIDs() {
		job := facts.LatestDone[kind]
		if job == nil {
			continue
		}
		counts := job.SeverityCounts()
		label := kindTitle(kind)
		switch {
		case job.Verdict == review.VerdictCritical || counts["critical"] > 0:
			add(LevelCrit, "review_"+kind, fmt.Sprintf("%s: %s", label, nonEmpty(job.Summary, "critical findings")))
		case job.Verdict == review.VerdictAttention || counts["high"] > 0:
			add(LevelWarn, "review_"+kind, fmt.Sprintf("%s: %s", label, nonEmpty(job.Summary, "needs attention")))
		}
	}
	for kind, schedule := range facts.Project.Schedule {
		period := schedulePeriod(schedule)
		if period == 0 {
			continue
		}
		last := facts.LatestDone[kind]
		if last == nil || facts.Now.Sub(last.CreatedAt) > 2*period {
			if attempt := facts.LastAttempts[kind]; attempt != nil && attempt.Status == review.StatusFailed {
				add(LevelWarn, "review_failed_"+kind, fmt.Sprintf("%s: last scheduled run failed", kindTitle(kind)))
			}
		}
	}
	sortFlags(flags)
	return flags
}

// MaxLevel returns the most severe level among flags.
func MaxLevel(flags []Flag) string {
	level := LevelOK
	for _, flag := range flags {
		if levelRank(flag.Level) > levelRank(level) {
			level = flag.Level
		}
	}
	return level
}

// schedulePeriod converts a schedule value to a duration (0 = off).
func schedulePeriod(schedule string) time.Duration {
	switch schedule {
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

// SchedulePeriod is exported for the scheduler.
func SchedulePeriod(schedule string) time.Duration { return schedulePeriod(schedule) }

func levelRank(level string) int {
	switch level {
	case LevelCrit:
		return 3
	case LevelWarn:
		return 2
	case LevelInfo:
		return 1
	default:
		return 0
	}
}

func sortFlags(flags []Flag) {
	for i := 1; i < len(flags); i++ {
		for j := i; j > 0 && levelRank(flags[j].Level) > levelRank(flags[j-1].Level); j-- {
			flags[j], flags[j-1] = flags[j-1], flags[j]
		}
	}
}

func kindTitle(kind string) string {
	if definition, ok := review.KindByID(kind); ok {
		return definition.Title
	}
	return strings.Title(kind) //nolint:staticcheck // ASCII kind ids only
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
