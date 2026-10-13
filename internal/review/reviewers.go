package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/i18n"
	"github.com/ffqa/xpier-xplan/internal/project"
)

const (
	smokeToken   = "XPLAN-OK"
	smokePrompt  = "This is a connection test from xplan. Do not use any tools and do not read any files. Reply with exactly: " + smokeToken
	smokeTimeout = 2 * time.Minute
)

// errCanceled is returned to run() when the job is canceled while a
// reviewer is being chosen.
var errCanceled = errors.New("canceled")

// ReviewerLabel names the job's reviewer for people; an "auto" job that has
// not picked one yet reads as "auto (first working reviewer)".
func (j *Job) ReviewerLabel() string {
	if j.Reviewer == config.AutoReviewer {
		return i18n.T("auto (first working reviewer)")
	}
	return j.Reviewer
}

// TestReviewer runs the smoke test for one preset and records the result.
func (m *Manager) TestReviewer(ctx context.Context, name string) (ReviewerHealth, error) {
	def, ok := m.settings().Reviewers[name]
	if !ok {
		return ReviewerHealth{}, fmt.Errorf("%w: reviewer %q is not configured", project.ErrInvalid, name)
	}
	lock := m.testLock(name)
	lock.Lock()
	defer lock.Unlock()
	health := m.smokeTest(ctx, def)
	if ctx.Err() != nil {
		return health, ctx.Err()
	}
	m.recordHealth(name, health)
	return health, nil
}

// ReviewerHealth returns every recorded reviewer result.
func (m *Manager) ReviewerHealth() map[string]ReviewerHealth {
	return m.health.All()
}

// ForgetReviewerHealth drops a reviewer's recorded result.
func (m *Manager) ForgetReviewerHealth(name string) {
	if err := m.health.Forget(name); err != nil {
		m.logger.Printf("reviewer health save failed name=%s: %v", name, err)
	}
}

// AutoChoice tells which reviewer "auto" would use right now without running
// anything: the first installed candidate with a fresh success (verified), or
// else the first installed candidate that still has to pass a test.
func (m *Manager) AutoChoice() (string, bool) {
	now := time.Now()
	firstUntested := ""
	for _, name := range m.settings().AutoCandidates() {
		if !m.installed(name) {
			continue
		}
		health, known := m.health.Get(name)
		if known && health.Fresh(now) {
			if health.Status == HealthOK {
				return name, true
			}
			continue
		}
		if firstUntested == "" {
			firstUntested = name
		}
	}
	return firstUntested, false
}

// smokeTest asks the reviewer for a fixed token in an empty directory.
func (m *Manager) smokeTest(ctx context.Context, def config.ReviewerDef) ReviewerHealth {
	started := time.Now()
	failed := func(detail string) ReviewerHealth {
		return ReviewerHealth{Status: HealthError, Detail: detail, CheckedAt: time.Now().UTC(), Source: "test"}
	}
	dir, err := os.MkdirTemp("", "xplan-smoke-*")
	if err != nil {
		return failed(err.Error())
	}
	defer os.RemoveAll(dir)
	promptFile := filepath.Join(dir, "prompt.md")
	if err := os.WriteFile(promptFile, []byte(smokePrompt), 0o600); err != nil {
		return failed(err.Error())
	}
	argv, err := BuildArgv(def, ArgvVars{Prompt: smokePrompt, PromptFile: promptFile, Path: dir, Home: m.store.Home()})
	if err != nil {
		return failed(err.Error())
	}
	logPath := filepath.Join(dir, "smoke.log")
	result := runAgent(ctx, argv, dir, filepath.Join(dir, "smoke.out"), logPath, smokeTimeout)
	health := ReviewerHealth{CheckedAt: time.Now().UTC(), DurationMS: time.Since(started).Milliseconds(), Source: "test"}
	switch {
	case result.Err == nil && strings.Contains(result.Output, smokeToken):
		health.Status = HealthOK
	case result.Err == nil:
		health.Status = HealthError
		health.Detail = i18n.T("unexpected answer: %s", clip(lastLine(result.Output), 160))
	default:
		health.Status, health.Detail = classifyFailure(result, readText(logPath))
	}
	return health
}

// ensureUsable decides whether "auto" may use a reviewer, running the smoke
// test when there is no fresh result. It returns a short reason when not.
func (m *Manager) ensureUsable(ctx context.Context, name string) (bool, string) {
	def, ok := m.settings().Reviewers[name]
	if !ok || len(def.Command) == 0 {
		return false, i18n.T("not configured")
	}
	if !m.installed(name) {
		return false, HealthLabel(HealthMissing)
	}
	if usable, reason, known := m.freshVerdict(name); known {
		return usable, reason
	}
	lock := m.testLock(name)
	lock.Lock()
	defer lock.Unlock()
	if usable, reason, known := m.freshVerdict(name); known {
		return usable, reason // another job tested it while we waited
	}
	m.logger.Printf("reviewer smoke test name=%s", name)
	health := m.smokeTest(ctx, def)
	if ctx.Err() != nil {
		return false, errCanceled.Error()
	}
	m.recordHealth(name, health)
	return health.Status == HealthOK, HealthLabel(health.Status)
}

func (m *Manager) freshVerdict(name string) (usable bool, reason string, known bool) {
	health, ok := m.health.Get(name)
	if !ok || !health.Fresh(time.Now()) || health.Status == HealthMissing {
		return false, "", false
	}
	return health.Status == HealthOK, HealthLabel(health.Status), true
}

// runWithReviewer runs the job's reviewer. With "auto" it walks the
// candidates, skips reviewers that are missing or cannot log in, and moves on
// when a real run fails for one of those reasons.
func (m *Manager) runWithReviewer(ctx context.Context, job *Job, vars ArgvVars, outputPath, logPath string) (runResult, error) {
	auto := job.ReviewerRequested == config.AutoReviewer
	candidates := []string{job.Reviewer}
	if auto {
		candidates = m.settings().AutoCandidates()
	}
	for _, name := range candidates {
		if ctx.Err() != nil {
			return runResult{ExitCode: -1, Err: errCanceled}, nil
		}
		if auto {
			usable, reason := m.ensureUsable(ctx, name)
			if !usable {
				m.noteAttempt(job, name, reason)
				continue
			}
			m.update(job, func(j *Job) { j.Reviewer = name })
		}
		def, ok := m.settings().Reviewers[name]
		if !ok {
			return runResult{}, fmt.Errorf("%w: reviewer %q is not configured", project.ErrInvalid, name)
		}
		argv, err := BuildArgv(def, vars)
		if err != nil {
			return runResult{}, err
		}
		result := runAgent(ctx, argv, vars.Path, outputPath, logPath, m.settings().ReviewDeadlineFor(name))
		status := m.recordRun(ctx, name, result, logPath)
		if auto && ctx.Err() == nil && (status == HealthAuth || status == HealthMissing) {
			m.noteAttempt(job, name, HealthLabel(status))
			continue
		}
		return result, nil
	}
	if ctx.Err() != nil {
		return runResult{ExitCode: -1, Err: errCanceled}, nil
	}
	return runResult{}, errors.New(i18n.T("No working reviewer (%s). Open Settings → Reviewers to test them, or log in to one.", strings.Join(job.Attempts, "; ")))
}

// recordRun updates reviewer health from a real review. Only clear signals
// are recorded: a success, a missing binary or a login problem. A crash or a
// timeout on a large project says little about the reviewer itself.
func (m *Manager) recordRun(ctx context.Context, name string, result runResult, logPath string) string {
	if ctx.Err() != nil {
		return ""
	}
	if result.Err == nil {
		m.recordHealth(name, ReviewerHealth{Status: HealthOK, CheckedAt: time.Now().UTC(), Source: "run"})
		return HealthOK
	}
	status, detail := classifyFailure(result, readText(logPath))
	if status == HealthAuth || status == HealthMissing {
		m.recordHealth(name, ReviewerHealth{Status: status, Detail: detail, CheckedAt: time.Now().UTC(), Source: "run"})
	}
	return status
}

func (m *Manager) noteAttempt(job *Job, name, reason string) {
	m.update(job, func(j *Job) { j.Attempts = append(j.Attempts, name+": "+reason) })
}

func (m *Manager) recordHealth(name string, health ReviewerHealth) {
	if err := m.health.Record(name, health); err != nil {
		m.logger.Printf("reviewer health save failed name=%s: %v", name, err)
	}
}

func (m *Manager) installed(name string) bool {
	def, ok := m.settings().Reviewers[name]
	if !ok || len(def.Command) == 0 {
		return false
	}
	_, err := LookupAgent(def.Command[0])
	return err == nil
}

func (m *Manager) testLock(name string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.testLocks[name]
	if !ok {
		lock = &sync.Mutex{}
		m.testLocks[name] = lock
	}
	return lock
}

func readText(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}
