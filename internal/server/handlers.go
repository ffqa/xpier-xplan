package server

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/console"
	"github.com/ffqa/xpier-xplan/internal/project"
	"github.com/ffqa/xpier-xplan/internal/review"
	"github.com/ffqa/xpier-xplan/internal/scanner"
)

type reviewerInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Command     []string               `json:"command"`
	Installed   bool                   `json:"installed"`
	Health      *review.ReviewerHealth `json:"health,omitempty"`
	HealthLabel string                 `json:"health_label,omitempty"`
}

type autoChoice struct {
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	cfg := s.app.Config()
	autoName, autoVerified := s.app.Reviews.AutoChoice()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":          app.Version,
		"home":             s.app.Home,
		"statuses":         project.Statuses,
		"scheduled_kinds":  project.ScheduledKinds,
		"schedule_values":  project.ScheduleValues,
		"review_kinds":     review.LocalizedKinds(),
		"doc_kinds":        scanner.DocKinds,
		"reviewers":        s.reviewerInfos(cfg),
		"default_reviewer": cfg.DefaultReviewer,
		"reviewer_order":   cfg.AutoCandidates(),
		"auto_choice":      autoChoice{Name: autoName, Verified: autoVerified},
		"language":         cfg.Language,
		"report_language":  cfg.ReportLanguage,
		"commands":         console.LocalizedCommands(),
	})
}

// reviewerInfos lists presets in auto-pick order with install and health state.
func (s *Server) reviewerInfos(cfg *config.Config) []reviewerInfo {
	healthByName := s.app.Reviews.ReviewerHealth()
	reviewers := []reviewerInfo{}
	for _, name := range cfg.AutoCandidates() {
		def := cfg.Reviewers[name]
		installed := false
		if len(def.Command) > 0 {
			_, err := review.LookupAgent(def.Command[0])
			installed = err == nil
		}
		info := reviewerInfo{Name: name, Description: def.Description, Command: def.Command, Installed: installed}
		if health, ok := healthByName[name]; ok {
			info.Health = &health
			info.HealthLabel = review.HealthLabel(health.Status)
		}
		reviewers = append(reviewers, info)
	}
	return reviewers
}

// handleTestReviewer runs the smoke test for one preset (up to two minutes).
func (s *Server) handleTestReviewer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	health, err := s.app.Reviews.TestReviewer(r.Context(), name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "health": health, "health_label": review.HealthLabel(health.Status)})
}

// handleTestReviewers smoke-tests every installed preset in parallel.
func (s *Server) handleTestReviewers(w http.ResponseWriter, r *http.Request) {
	cfg := s.app.Config()
	results := map[string]review.ReviewerHealth{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, info := range s.reviewerInfos(cfg) {
		if !info.Installed {
			continue
		}
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			health, err := s.app.Reviews.TestReviewer(r.Context(), name)
			if err != nil {
				s.logger.Printf("reviewer test failed name=%s: %v", name, err)
				return
			}
			mu.Lock()
			results[name] = health
			mu.Unlock()
		}(info.Name)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "reviewers": s.reviewerInfos(cfg)})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Config())
}

type configPatch struct {
	DefaultReviewer      *string           `json:"default_reviewer"`
	ReviewerOrder        []string          `json:"reviewer_order"`
	Language             *string           `json:"language"`
	ReportLanguage       *string           `json:"report_language"`
	ScanInterval         *string           `json:"scan_interval"`
	ReviewTimeout        *string           `json:"review_timeout"`
	MaxConcurrentReviews *int              `json:"max_concurrent_reviews"`
	DefaultSchedule      map[string]string `json:"default_schedule"`
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch configPatch
	if err := decodeBody(r, &patch); err != nil {
		s.writeError(w, err)
		return
	}
	updated, err := s.app.UpdateConfig(func(cfg *config.Config) error {
		if patch.DefaultReviewer != nil {
			cfg.DefaultReviewer = strings.TrimSpace(*patch.DefaultReviewer)
		}
		if patch.ReviewerOrder != nil {
			cfg.ReviewerOrder = patch.ReviewerOrder
		}
		if patch.Language != nil {
			cfg.Language = strings.TrimSpace(*patch.Language)
		}
		if patch.ReportLanguage != nil {
			cfg.ReportLanguage = strings.TrimSpace(*patch.ReportLanguage)
		}
		for _, field := range []struct {
			value  *string
			target *string
		}{{patch.ScanInterval, &cfg.ScanInterval}, {patch.ReviewTimeout, &cfg.ReviewTimeout}} {
			if field.value == nil {
				continue
			}
			if parsed, err := time.ParseDuration(*field.value); err != nil || parsed < time.Minute {
				return fmt.Errorf("%w: durations must look like 15m or 1h and be at least 1m", project.ErrInvalid)
			}
			*field.target = *field.value
		}
		if patch.MaxConcurrentReviews != nil {
			if *patch.MaxConcurrentReviews < 1 || *patch.MaxConcurrentReviews > 8 {
				return fmt.Errorf("%w: max_concurrent_reviews must be 1-8", project.ErrInvalid)
			}
			cfg.MaxConcurrentReviews = *patch.MaxConcurrentReviews
		}
		for kind, value := range patch.DefaultSchedule {
			if !contains(project.ScheduledKinds, kind) || !contains(project.ScheduleValues, value) {
				return fmt.Errorf("%w: schedule %s=%s not allowed", project.ErrInvalid, kind, value)
			}
			cfg.DefaultSchedule[kind] = value
		}
		return nil
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	cards, err := s.app.Overview()
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cards": cards, "active_reviews": s.app.Reviews.Active()})
}

type inboxItem struct {
	Project     string         `json:"project"`
	ProjectName string         `json:"project_name"`
	Kind        string         `json:"kind"`
	JobID       string         `json:"job_id"`
	Verdict     string         `json:"verdict"`
	At          time.Time      `json:"at"`
	Finding     review.Finding `json:"finding"`
}

var severityRank = map[string]int{"critical": 4, "high": 3, "medium": 2, "low": 1, "info": 0}

// handleInbox lists findings from the latest finished review of every kind,
// across all projects, most severe first.
func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	projectList, err := s.app.Store.List()
	if err != nil {
		s.writeError(w, err)
		return
	}
	minimum := severityRank[r.URL.Query().Get("min")]
	items := []inboxItem{}
	for _, item := range projectList {
		for kind, job := range s.app.Reviews.LatestDone(item.Slug) {
			for _, finding := range job.Findings {
				if severityRank[finding.Severity] < minimum {
					continue
				}
				items = append(items, inboxItem{Project: item.Slug, ProjectName: item.Name, Kind: kind, JobID: job.ID, Verdict: job.Verdict, At: job.CreatedAt, Finding: finding})
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if severityRank[items[i].Finding.Severity] != severityRank[items[j].Finding.Severity] {
			return severityRank[items[i].Finding.Severity] > severityRank[items[j].Finding.Severity]
		}
		return items[i].At.After(items[j].At)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleActiveReviews(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.app.Reviews.Active()})
}

func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Root string `json:"root"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	if strings.TrimSpace(body.Root) == "" {
		s.writeError(w, fmt.Errorf("%w: root is required", project.ErrInvalid))
		return
	}
	candidates, err := s.app.Discover(body.Root)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": candidates})
}

func (s *Server) handleAddProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	created, err := s.app.Store.Add(body.Path, strings.TrimSpace(body.Name))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if _, err := s.app.ScanProject(created.Slug); err != nil {
		s.logger.Printf("initial scan of %s failed: %v", created.Slug, err)
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	found, err := s.app.Store.Get(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, found)
}

func (s *Server) handlePatchProject(w http.ResponseWriter, r *http.Request) {
	var patch project.Patch
	if err := decodeBody(r, &patch); err != nil {
		s.writeError(w, err)
		return
	}
	if patch.Reviewer != nil && *patch.Reviewer != "" {
		if !s.app.Config().IsReviewer(*patch.Reviewer) {
			s.writeError(w, fmt.Errorf("%w: reviewer %q is not configured", project.ErrInvalid, *patch.Reviewer))
			return
		}
	}
	updated, err := s.app.Store.Update(r.PathValue("slug"), patch)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Store.Remove(r.PathValue("slug")); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := s.app.Brief(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, brief)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.app.ScanProject(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scanned_at":  snapshot.ScannedAt,
		"duration_ms": snapshot.DurationMs,
		"docs":        len(snapshot.Docs),
		"errors":      snapshot.Errors,
	})
}

func (s *Server) handleVisit(w http.ResponseWriter, r *http.Request) {
	state, err := s.app.MarkCaughtUp(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if _, err := s.app.Store.Get(slug); err != nil {
		s.writeError(w, err)
		return
	}
	snapshot, err := scanner.LoadLatest(s.app.Store.Dir(slug))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if snapshot == nil {
		if snapshot, err = s.app.ScanProject(slug); err != nil {
			s.writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"docs":       snapshot.Docs,
		"kinds":      scanner.DocKinds,
		"truncated":  snapshot.DocsTruncated,
		"scanned_at": snapshot.ScannedAt,
	})
}

func (s *Server) projectRoot(w http.ResponseWriter, r *http.Request) (string, bool) {
	found, err := s.app.Store.Get(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return "", false
	}
	return found.Path, true
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	root, ok := s.projectRoot(w, r)
	if !ok {
		return
	}
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		s.writeError(w, fmt.Errorf("%w: path is required", project.ErrInvalid))
		return
	}
	content, err := scanner.ReadText(root, relPath)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": relPath, "kind": scanner.ClassifyDoc(relPath), "content": content})
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	root, ok := s.projectRoot(w, r)
	if !ok {
		return
	}
	entries, err := scanner.ListDir(root, r.URL.Query().Get("path"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleGetIntent(w http.ResponseWriter, r *http.Request) {
	content, err := s.app.Store.ReadIntent(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, intentPayload(content))
}

func (s *Server) handlePutIntent(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	if _, err := s.app.Store.Get(slug); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.app.Store.WriteIntent(slug, body.Content); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, intentPayload(body.Content))
}

func intentPayload(content string) map[string]any {
	return map[string]any{
		"content": content,
		"filled":  project.IntentFilled(content),
		"draft":   project.IntentIsDraft(content),
	}
}

func (s *Server) handleDraftIntents(w http.ResponseWriter, r *http.Request) {
	queued, skipped, err := s.app.DraftMissingIntents()
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": queued, "skipped": skipped})
}

func (s *Server) handleListADRs(w http.ResponseWriter, r *http.Request) {
	adrs, err := s.app.Store.ListADRs(r.PathValue("slug"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"adrs": adrs})
}

func (s *Server) handleCreateADR(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title   string `json:"title"`
		Context string `json:"context"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	created, err := s.app.Store.CreateADR(r.PathValue("slug"), body.Title, body.Context)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateADR(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	updated, err := s.app.Store.UpdateADR(r.PathValue("slug"), r.PathValue("id"), body.Content)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleListJournal(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 100)
	entries, err := s.app.Store.ReadJournal(r.PathValue("slug"), r.URL.Query().Get("kind"), limit)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleAddJournal(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var entry project.Entry
	if err := decodeBody(r, &entry); err != nil {
		s.writeError(w, err)
		return
	}
	var saved project.Entry
	var err error
	if entry.Kind == project.EntryPark {
		if _, getErr := s.app.Store.Get(slug); getErr != nil {
			s.writeError(w, getErr)
			return
		}
		saved, err = s.app.Park(slug, entry)
	} else {
		saved, err = s.app.Store.AppendJournal(slug, entry)
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if _, err := s.app.Store.Get(slug); err != nil {
		s.writeError(w, err)
		return
	}
	jobs, err := s.app.Reviews.List(slug, queryInt(r, "limit", 100))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) handleStartReview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		Question string `json:"question"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	job, err := s.app.Reviews.Enqueue(r.PathValue("slug"), body.Kind, body.Question, "manual")
	if errors.Is(err, review.ErrAlreadyActive) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "job": job})
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	slug, id := r.PathValue("slug"), r.PathValue("id")
	if _, err := s.app.Store.Get(slug); err != nil {
		s.writeError(w, err)
		return
	}
	job, err := s.app.Reviews.Get(slug, id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	report, err := s.app.Reviews.Output(slug, id)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job, "report": report})
}

func (s *Server) handleCancelReview(w http.ResponseWriter, r *http.Request) {
	slug, id := r.PathValue("slug"), r.PathValue("id")
	if _, err := s.app.Store.Get(slug); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.app.Reviews.Cancel(slug, id); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "canceling"})
}

func (s *Server) handleConsoleHistory(w http.ResponseWriter, r *http.Request) {
	items, err := s.console.History(r.PathValue("slug"), queryInt(r, "limit", 50))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleConsoleRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input string `json:"input"`
	}
	if err := decodeBody(r, &body); err != nil {
		s.writeError(w, err)
		return
	}
	result, err := s.console.Run(r.PathValue("slug"), body.Input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryInt(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
