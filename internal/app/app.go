// Package app wires the store, scanner, review manager and insight views into
// one service used by the HTTP server, the console and the CLI.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/i18n"
	"github.com/ffqa/xpier-xplan/internal/insight"
	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/review"
	"github.com/ffqa/xpier-xplan/internal/scanner"
)

// Version is overridden at build time via -ldflags.
var Version = "0.4.0"

// App is the shared service.
type App struct {
	Home    string
	Store   *project.Store
	Reviews *review.Manager
	Logger  *log.Logger

	cfgMu sync.RWMutex
	cfg   *config.Config

	scanMu sync.Mutex
}

// New loads config and builds the service.
func New(ctx context.Context, home string, logger *log.Logger) (*App, error) {
	cfg, err := config.Load(home)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureAgentFiles(home); err != nil {
		return nil, err
	}
	i18n.SetLanguage(cfg.Language)
	application := &App{Home: home, Logger: logger, cfg: cfg}
	application.Store = project.NewStore(home, cfg.DefaultSchedule)
	application.Reviews = review.NewManager(ctx, application.Store, application.Config, logger)
	return application, nil
}

// Config returns the current configuration.
func (a *App) Config() *config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

// UpdateConfig validates, saves and swaps the configuration.
func (a *App) UpdateConfig(mutate func(*config.Config) error) (*config.Config, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	next := *a.cfg
	next.DefaultSchedule = copyMap(a.cfg.DefaultSchedule)
	next.Reviewers = map[string]config.ReviewerDef{}
	for name, def := range a.cfg.Reviewers {
		next.Reviewers[name] = def
	}
	next.ReviewerOrder = append([]string(nil), a.cfg.ReviewerOrder...)
	if err := mutate(&next); err != nil {
		return nil, err
	}
	if !next.IsReviewer(next.DefaultReviewer) {
		return nil, fmt.Errorf("%w: default reviewer %q is not configured", project.ErrInvalid, next.DefaultReviewer)
	}
	if !config.IsLanguage(next.Language) {
		return nil, fmt.Errorf("%w: language must be one of %v", project.ErrInvalid, config.Languages)
	}
	for _, name := range next.ReviewerOrder {
		if _, ok := next.Reviewers[name]; !ok {
			return nil, fmt.Errorf("%w: reviewer_order contains unknown reviewer %q", project.ErrInvalid, name)
		}
	}
	if err := config.Save(a.Home, &next); err != nil {
		return nil, err
	}
	a.cfg = &next
	i18n.SetLanguage(next.Language)
	return a.cfg, nil
}

// ScanProject scans one project and persists the snapshot.
func (a *App) ScanProject(slug string) (*scanner.Snapshot, error) {
	target, err := a.Store.Get(slug)
	if err != nil {
		return nil, err
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	snapshot := scanner.Scan(target.Path, target.DocGlobs)
	if err := scanner.Save(a.Store.Dir(slug), snapshot); err != nil {
		return nil, err
	}
	scannedAt := snapshot.ScannedAt
	if _, err := a.Store.UpdateState(slug, func(state *project.State) { state.LastScanAt = &scannedAt }); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// ScanAll scans every project sequentially and returns per-project errors.
func (a *App) ScanAll() map[string]error {
	failures := map[string]error{}
	projectList, err := a.Store.List()
	if err != nil {
		failures["*"] = err
		return failures
	}
	for _, item := range projectList {
		if _, err := a.ScanProject(item.Slug); err != nil {
			failures[item.Slug] = err
		}
	}
	return failures
}

// Facts collects the inputs for health/cards/briefs.
func (a *App) Facts(target *project.Project) (insight.Facts, error) {
	snapshot, err := scanner.LoadLatest(a.Store.Dir(target.Slug))
	if err != nil {
		return insight.Facts{}, err
	}
	state, err := a.Store.ReadState(target.Slug)
	if err != nil {
		return insight.Facts{}, err
	}
	intent, err := a.Store.ReadIntent(target.Slug)
	if err != nil {
		return insight.Facts{}, err
	}
	lastPark, err := a.Store.LastPark(target.Slug)
	if err != nil {
		return insight.Facts{}, err
	}
	jobs, err := a.Reviews.List(target.Slug, 0)
	if err != nil {
		return insight.Facts{}, err
	}
	latestDone := map[string]*review.Job{}
	lastAttempts := map[string]*review.Job{}
	activeJobs := 0
	for _, job := range jobs {
		if job.Active() {
			activeJobs++
		}
		if !review.IsReviewKind(job.Kind) {
			continue
		}
		if lastAttempts[job.Kind] == nil {
			lastAttempts[job.Kind] = job
		}
		if job.Status == review.StatusDone && latestDone[job.Kind] == nil {
			latestDone[job.Kind] = job
		}
	}
	return insight.Facts{
		Project:      target,
		Snapshot:     snapshot,
		State:        state,
		IntentFilled: project.IntentFilled(intent),
		IntentDraft:  project.IntentIsDraft(intent),
		LastPark:     lastPark,
		LatestDone:   latestDone,
		LastAttempts: lastAttempts,
		ActiveJobs:   activeJobs,
		Now:          time.Now().UTC(),
	}, nil
}

// Overview returns a card for every project.
func (a *App) Overview() ([]insight.Card, error) {
	projectList, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	cards := make([]insight.Card, 0, len(projectList))
	for _, item := range projectList {
		facts, err := a.Facts(item)
		if err != nil {
			a.Logger.Printf("overview: facts for %s failed: %v", item.Slug, err)
			continue
		}
		cards = append(cards, insight.BuildCard(facts))
	}
	return cards, nil
}

// Brief builds the re-entry brief for a project.
func (a *App) Brief(slug string) (*insight.Brief, error) {
	target, err := a.Store.Get(slug)
	if err != nil {
		return nil, err
	}
	facts, err := a.Facts(target)
	if err != nil {
		return nil, err
	}
	intent, err := a.Store.ReadIntent(slug)
	if err != nil {
		return nil, err
	}
	adrs, err := a.Store.ListADRs(slug)
	if err != nil {
		return nil, err
	}
	jobs, err := a.Reviews.List(slug, 50)
	if err != nil {
		return nil, err
	}
	brief := insight.BuildBrief(insight.BriefInput{Facts: facts, Intent: intent, ADRs: adrs, Reviews: jobs})
	return &brief, nil
}

// DraftOutcome reports what DraftMissingIntents did for one project. Code is
// queued, filled, new, running or error.
type DraftOutcome struct {
	Project string `json:"project"`
	Code    string `json:"code"`
	JobID   string `json:"job_id,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// DraftMissingIntents queues an intent draft for every project that existed
// before it was registered and still has no intent. Projects started together
// with their xplan entry are skipped on purpose: their owner writes the intent.
func (a *App) DraftMissingIntents() (queued, skipped []DraftOutcome, err error) {
	projectList, err := a.Store.List()
	if err != nil {
		return nil, nil, err
	}
	queued, skipped = []DraftOutcome{}, []DraftOutcome{}
	for _, item := range projectList {
		outcome := a.draftIntentIfMissing(item)
		if outcome.Code == "queued" {
			queued = append(queued, outcome)
			continue
		}
		skipped = append(skipped, outcome)
	}
	return queued, skipped, nil
}

func (a *App) draftIntentIfMissing(target *project.Project) DraftOutcome {
	outcome := DraftOutcome{Project: target.Slug}
	facts, err := a.Facts(target)
	if err != nil {
		outcome.Code, outcome.Reason = "error", err.Error()
		return outcome
	}
	if facts.IntentFilled {
		outcome.Code, outcome.Reason = "filled", i18n.T("intent already written")
		return outcome
	}
	if !insight.Established(target, facts.Snapshot) {
		outcome.Code, outcome.Reason = "new", i18n.T("started together with its xplan entry: write this one yourself")
		return outcome
	}
	job, err := a.Reviews.Enqueue(target.Slug, review.KindIntent, "", "bulk")
	switch {
	case errors.Is(err, review.ErrAlreadyActive):
		outcome.Code, outcome.Reason, outcome.JobID = "running", i18n.T("a draft is already running"), job.ID
	case err != nil:
		outcome.Code, outcome.Reason = "error", err.Error()
	default:
		outcome.Code, outcome.JobID = "queued", job.ID
	}
	return outcome
}

// CurrentHead returns the project's HEAD from the latest snapshot.
func (a *App) CurrentHead(slug string) string {
	snapshot, err := scanner.LoadLatest(a.Store.Dir(slug))
	if err != nil || snapshot == nil || snapshot.Git == nil {
		return ""
	}
	return snapshot.Git.Head
}

// Park appends a park note, stamping the current HEAD, and records a visit.
func (a *App) Park(slug string, entry project.Entry) (project.Entry, error) {
	if _, err := a.ScanProject(slug); err != nil {
		a.Logger.Printf("park: rescan %s failed: %v", slug, err)
	}
	entry.Kind = project.EntryPark
	entry.Head = a.CurrentHead(slug)
	saved, err := a.Store.AppendJournal(slug, entry)
	if err != nil {
		return project.Entry{}, err
	}
	if _, err := a.Store.MarkVisit(slug, entry.Head); err != nil {
		return project.Entry{}, err
	}
	return saved, nil
}

// MarkCaughtUp records that the user has seen the current state.
func (a *App) MarkCaughtUp(slug string) (project.State, error) {
	if _, err := a.ScanProject(slug); err != nil {
		a.Logger.Printf("caught-up: rescan %s failed: %v", slug, err)
	}
	return a.Store.MarkVisit(slug, a.CurrentHead(slug))
}

// Discover lists git repositories directly under root that are not registered.
func (a *App) Discover(root string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(absRoot, "*", ".git"))
	if err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	projectList, err := a.Store.List()
	if err != nil {
		return nil, err
	}
	for _, item := range projectList {
		registered[item.Path] = true
	}
	candidates := []string{}
	for _, gitDir := range matches {
		dir := filepath.Dir(gitDir)
		if !registered[dir] {
			candidates = append(candidates, dir)
		}
	}
	return candidates, nil
}

func copyMap(source map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range source {
		result[key] = value
	}
	return result
}
