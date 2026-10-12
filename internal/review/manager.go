package review

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xplan/internal/config"
	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/scanner"
)

// ErrAlreadyActive is returned when the same review kind is already queued
// or running for a project.
var ErrAlreadyActive = errors.New("a review of this kind is already queued or running")

// ErrJobNotFound is returned for unknown job IDs.
var ErrJobNotFound = errors.New("review job not found")

// Manager queues, runs and persists review jobs.
type Manager struct {
	store    *project.Store
	settings func() *config.Config
	logger   *log.Logger

	slots   chan struct{}
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	baseCtx context.Context
}

// NewManager creates a manager; settings is read at run time so config edits
// apply without a restart.
func NewManager(ctx context.Context, store *project.Store, settings func() *config.Config, logger *log.Logger) *Manager {
	slots := settings().MaxConcurrentReviews
	if slots < 1 {
		slots = 1
	}
	return &Manager{
		store:    store,
		settings: settings,
		logger:   logger,
		slots:    make(chan struct{}, slots),
		cancels:  map[string]context.CancelFunc{},
		baseCtx:  ctx,
	}
}

// ReviewerFor returns the reviewer name configured for a project.
func (m *Manager) ReviewerFor(p *project.Project) string {
	if p.Reviewer != "" {
		return p.Reviewer
	}
	return m.settings().DefaultReviewer
}

// Enqueue validates and schedules a job. trigger is manual|schedule|console.
func (m *Manager) Enqueue(slug, kindID, question, trigger string) (*Job, error) {
	target, err := m.store.Get(slug)
	if err != nil {
		return nil, err
	}
	kind, ok := KindByID(kindID)
	if !ok {
		return nil, fmt.Errorf("%w: unknown review kind %q", project.ErrInvalid, kindID)
	}
	if kind.ID == KindAsk && strings.TrimSpace(question) == "" {
		return nil, fmt.Errorf("%w: a question is required", project.ErrInvalid)
	}
	reviewerName := m.ReviewerFor(target)
	if _, ok := m.settings().Reviewers[reviewerName]; !ok {
		return nil, fmt.Errorf("%w: reviewer %q is not configured", project.ErrInvalid, reviewerName)
	}

	m.mu.Lock()
	if kind.ID != KindAsk {
		active, err := m.activeJob(slug, kind.ID)
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		if active != nil {
			m.mu.Unlock()
			return active, ErrAlreadyActive
		}
	}
	now := time.Now().UTC()
	job := &Job{
		ID:        newJobID(now, kind.ID),
		Project:   slug,
		Kind:      kind.ID,
		Question:  strings.TrimSpace(question),
		Reviewer:  reviewerName,
		Trigger:   trigger,
		Status:    StatusQueued,
		CreatedAt: now,
		Findings:  []Finding{},
	}
	jobCtx, cancel := context.WithCancel(m.baseCtx)
	m.cancels[job.ID] = cancel
	saveErr := m.save(job)
	m.mu.Unlock()
	if saveErr != nil {
		cancel()
		return nil, saveErr
	}

	created := *job
	go m.run(jobCtx, target, kind, job)
	return &created, nil
}

// run waits for a slot, executes the agent and records the outcome.
func (m *Manager) run(ctx context.Context, target *project.Project, kind Kind, job *Job) {
	defer m.forget(job.ID)

	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		m.finish(job, func(j *Job) { j.Status = StatusCanceled; j.Error = "canceled before start" })
		return
	}

	started := time.Now().UTC()
	m.update(job, func(j *Job) { j.Status = StatusRunning; j.StartedAt = &started })
	m.logger.Printf("review start project=%s kind=%s id=%s reviewer=%s", job.Project, job.Kind, job.ID, job.Reviewer)

	prompt, err := m.buildPrompt(target, kind, job)
	if err != nil {
		m.finish(job, func(j *Job) { j.Status = StatusFailed; j.Error = err.Error() })
		return
	}
	m.update(job, func(j *Job) { j.PromptChars = len(prompt) })
	reviewsDir := filepath.Join(m.store.Dir(job.Project), "reviews")
	promptFile := filepath.Join(reviewsDir, job.ID+".prompt.md")
	if err := config.WriteFileAtomic(promptFile, []byte(prompt)); err != nil {
		m.finish(job, func(j *Job) { j.Status = StatusFailed; j.Error = err.Error() })
		return
	}
	def := m.settings().Reviewers[job.Reviewer]
	argv, err := BuildArgv(def, prompt, promptFile, target.Path)
	if err != nil {
		m.finish(job, func(j *Job) { j.Status = StatusFailed; j.Error = err.Error() })
		return
	}

	isRepo := scanner.IsRepo(target.Path)
	fingerprintBefore := ""
	if isRepo {
		fingerprintBefore = scanner.StatusFingerprint(target.Path)
	}
	result := runAgent(ctx, argv, target.Path,
		filepath.Join(reviewsDir, job.ID+".md"),
		filepath.Join(reviewsDir, job.ID+".log"),
		m.settings().ReviewDeadline())
	treeChanged := isRepo && scanner.StatusFingerprint(target.Path) != fingerprintBefore

	var draft draftOutcome
	var draftErr error
	if kind.ID == KindIntent && result.Err == nil {
		draft, draftErr = m.applyIntentDraft(target, job, result.Output, filepath.Join(reviewsDir, job.ID+".md"))
	}

	m.finish(job, func(j *Job) {
		j.PromptChars = len(prompt)
		j.ExitCode = result.ExitCode
		j.TreeChanged = treeChanged
		report, _, parsed := ParseReport(result.Output)
		if kind.Review {
			j.Verdict = report.Verdict
			j.Score = report.Score
			j.Summary = report.Summary
			j.Findings = report.Findings
		}
		if treeChanged {
			j.Findings = append(j.Findings, Finding{
				Severity: "info",
				Title:    "Working tree changed while the review was running",
				Detail:   "git status differs before and after the run. It may be your own parallel work, or the agent did not respect read-only mode. Check `git status` in the project.",
			})
		}
		switch {
		case result.Err != nil && errors.Is(ctx.Err(), context.Canceled):
			j.Status = StatusCanceled
			j.Error = "canceled"
		case result.Err != nil && strings.TrimSpace(result.Output) == "":
			j.Status = StatusFailed
			j.Error = result.Err.Error() + tailOfLog(filepath.Join(reviewsDir, job.ID+".log"))
		case result.Err != nil:
			j.Status = StatusFailed
			j.Error = result.Err.Error()
		case draftErr != nil:
			j.Status = StatusFailed
			j.Error = draftErr.Error()
		default:
			j.Status = StatusDone
			j.Summary = nonEmpty(draft.summary, j.Summary)
			j.IntentWritten = draft.written
			if kind.Review && !parsed {
				j.Error = "report has no parsable JSON block; verdict unknown"
			}
		}
	})
	m.logger.Printf("review end project=%s kind=%s id=%s status=%s verdict=%s", job.Project, job.Kind, job.ID, job.Status, job.Verdict)
}

func (m *Manager) buildPrompt(target *project.Project, kind Kind, job *Job) (string, error) {
	intent, err := m.store.ReadIntent(target.Slug)
	if err != nil {
		return "", err
	}
	adrs, err := m.store.ListADRs(target.Slug)
	if err != nil {
		return "", err
	}
	lastPark, err := m.store.LastPark(target.Slug)
	if err != nil {
		return "", err
	}
	snapshot, err := scanner.LoadLatest(m.store.Dir(target.Slug))
	if err != nil {
		return "", err
	}
	var previous *Job
	if kind.Review {
		previous = m.LatestDone(target.Slug)[kind.ID]
	}
	var firstCommits []scanner.Commit
	if kind.ID == KindIntent {
		firstCommits = scanner.FirstCommits(target.Path, maxFirstCommits)
	}
	language := m.settings().ReportLanguage
	if language == "" {
		language = "English"
	}
	return BuildPrompt(PromptInput{
		Project:      target,
		Kind:         kind,
		Question:     job.Question,
		Intent:       intent,
		ADRs:         adrs,
		LastPark:     lastPark,
		Snapshot:     snapshot,
		PreviousJob:  previous,
		Language:     language,
		FirstCommits: firstCommits,
	}), nil
}

type draftOutcome struct {
	summary string
	written bool
}

// applyIntentDraft turns an intent job's answer into a clean draft (which
// becomes the job's report) and writes it to intent.md only while the owner
// has not written an intent: their own words are never overwritten.
func (m *Manager) applyIntentDraft(target *project.Project, job *Job, output, reportPath string) (draftOutcome, error) {
	draft, err := ExtractIntentDraft(output, target.Name, job.Reviewer, time.Now())
	if err != nil {
		return draftOutcome{}, err
	}
	if err := config.WriteFileAtomic(reportPath, []byte(draft)); err != nil {
		return draftOutcome{}, err
	}
	current, err := m.store.ReadIntent(target.Slug)
	if err != nil {
		return draftOutcome{}, err
	}
	if project.IntentFilled(current) {
		return draftOutcome{summary: "Draft ready. Your intent was kept as it is; load the draft in the Intent tab to compare."}, nil
	}
	if err := m.store.WriteIntent(target.Slug, draft); err != nil {
		return draftOutcome{}, err
	}
	m.logger.Printf("intent draft written project=%s id=%s", target.Slug, job.ID)
	return draftOutcome{summary: "Draft written to intent.md. Correct it in your own words, then delete the Draft line.", written: true}, nil
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// Cancel stops a queued or running job.
func (m *Manager) Cancel(slug, id string) error {
	job, err := m.Get(slug, id)
	if err != nil {
		return err
	}
	if !job.Active() {
		return fmt.Errorf("%w: job is already %s", project.ErrInvalid, job.Status)
	}
	m.mu.Lock()
	cancel, ok := m.cancels[id]
	m.mu.Unlock()
	if !ok {
		// Orphaned from a previous process: mark it directly.
		m.finish(job, func(j *Job) { j.Status = StatusCanceled; j.Error = "canceled" })
		return nil
	}
	cancel()
	return nil
}

// Get loads one job.
func (m *Manager) Get(slug, id string) (*Job, error) {
	if !validJobID(id) {
		return nil, ErrJobNotFound
	}
	raw, err := os.ReadFile(filepath.Join(m.store.Dir(slug), "reviews", id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read job %s: %w", id, err)
	}
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, fmt.Errorf("parse job %s: %w", id, err)
	}
	return &job, nil
}

// Output returns the report Markdown (JSON block stripped) for a job.
func (m *Manager) Output(slug, id string) (string, error) {
	if !validJobID(id) {
		return "", ErrJobNotFound
	}
	raw, err := os.ReadFile(filepath.Join(m.store.Dir(slug), "reviews", id+".md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read report %s: %w", id, err)
	}
	_, cleaned, _ := ParseReport(string(raw))
	return cleaned, nil
}

// List returns a project's jobs, newest first.
func (m *Manager) List(slug string, limit int) ([]*Job, error) {
	dir := filepath.Join(m.store.Dir(slug), "reviews")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []*Job{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	ids := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	jobs := []*Job{}
	for _, id := range ids {
		job, err := m.Get(slug, id)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
		if limit > 0 && len(jobs) >= limit {
			break
		}
	}
	return jobs, nil
}

// LatestDone returns the most recent finished job per review kind.
func (m *Manager) LatestDone(slug string) map[string]*Job {
	latest := map[string]*Job{}
	jobs, err := m.List(slug, 0)
	if err != nil {
		return latest
	}
	for _, job := range jobs {
		if job.Status == StatusDone && IsReviewKind(job.Kind) && latest[job.Kind] == nil {
			latest[job.Kind] = job
		}
	}
	return latest
}

// LastAttempt returns the newest job of a kind regardless of status.
func (m *Manager) LastAttempt(slug, kind string) *Job {
	jobs, err := m.List(slug, 0)
	if err != nil {
		return nil
	}
	for _, job := range jobs {
		if job.Kind == kind {
			return job
		}
	}
	return nil
}

// Active returns every queued or running job across all projects.
func (m *Manager) Active() []*Job {
	projectList, err := m.store.List()
	if err != nil {
		return []*Job{}
	}
	active := []*Job{}
	for _, item := range projectList {
		jobs, err := m.List(item.Slug, 20)
		if err != nil {
			continue
		}
		for _, job := range jobs {
			if job.Active() {
				active = append(active, job)
			}
		}
	}
	return active
}

// RecoverInterrupted marks jobs left active by a previous process as failed.
func (m *Manager) RecoverInterrupted() {
	for _, job := range m.Active() {
		m.mu.Lock()
		_, live := m.cancels[job.ID]
		m.mu.Unlock()
		if live {
			continue
		}
		m.finish(job, func(j *Job) {
			j.Status = StatusFailed
			j.Error = "interrupted: xplan stopped while this review was running"
		})
	}
}

func (m *Manager) activeJob(slug, kind string) (*Job, error) {
	jobs, err := m.List(slug, 30)
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job.Kind == kind && job.Active() {
			return job, nil
		}
	}
	return nil, nil
}

func (m *Manager) update(job *Job, mutate func(*Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mutate(job)
	if err := m.save(job); err != nil {
		m.logger.Printf("review save failed id=%s: %v", job.ID, err)
	}
}

func (m *Manager) finish(job *Job, mutate func(*Job)) {
	m.update(job, func(j *Job) {
		mutate(j)
		finished := time.Now().UTC()
		j.FinishedAt = &finished
	})
}

func (m *Manager) forget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.cancels[id]; ok {
		cancel()
		delete(m.cancels, id)
	}
}

func (m *Manager) save(job *Job) error {
	raw, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("encode job: %w", err)
	}
	return config.WriteFileAtomic(filepath.Join(m.store.Dir(job.Project), "reviews", job.ID+".json"), raw)
}

var jobIDPattern = regexp.MustCompile(`^\d{8}-\d{6}-[a-z]+-[0-9a-f]{4}$`)

func validJobID(id string) bool { return jobIDPattern.MatchString(id) }

func newJobID(now time.Time, kind string) string {
	suffix := make([]byte, 2)
	if _, err := rand.Read(suffix); err != nil {
		suffix = []byte{byte(now.Nanosecond()), byte(now.Nanosecond() >> 8)}
	}
	return fmt.Sprintf("%s-%s-%s", now.Local().Format("20060102-150405"), kind, hex.EncodeToString(suffix))
}

func tailOfLog(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return ""
	}
	text := strings.TrimSpace(string(raw))
	if len(text) > 600 {
		text = "…" + text[len(text)-600:]
	}
	return "\n" + text
}
