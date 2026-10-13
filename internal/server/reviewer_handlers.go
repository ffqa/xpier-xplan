package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"github.com/ffqa/xpier-xplan/internal/i18n"
	"github.com/ffqa/xpier-xplan/internal/review"
)

type reviewerInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Command     []string `json:"command"`
	Model       string   `json:"model"`
	Agent       string   `json:"agent"`
	Timeout     string   `json:"timeout"`
	// EffectiveTimeout is the reviewer's own timeout or the global one.
	EffectiveTimeout string                 `json:"effective_timeout"`
	SupportsModel    bool                   `json:"supports_model"`
	SupportsAgent    bool                   `json:"supports_agent"`
	Builtin          bool                   `json:"builtin"`
	Modified         bool                   `json:"modified"`
	Installed        bool                   `json:"installed"`
	Health           *review.ReviewerHealth `json:"health,omitempty"`
	HealthLabel      string                 `json:"health_label,omitempty"`
}

// reviewerInfos lists reviewers in auto-pick order with install and health state.
func (s *Server) reviewerInfos(cfg *config.Config) []reviewerInfo {
	healthByName := s.app.Reviews.ReviewerHealth()
	builtins := config.BuiltinReviewers()
	reviewers := []reviewerInfo{}
	for _, name := range cfg.AutoCandidates() {
		def := cfg.Reviewers[name]
		preset, isBuiltin := builtins[name]
		info := reviewerInfo{
			Name:             name,
			Description:      def.Description,
			Command:          def.Command,
			Model:            def.Model,
			Agent:            def.Agent,
			Timeout:          def.Timeout,
			EffectiveTimeout: shortDuration(cfg.ReviewDeadlineFor(name)),
			SupportsModel:    def.Supports(config.SlotModel),
			SupportsAgent:    def.Supports(config.SlotAgent),
			Builtin:          isBuiltin,
			Modified:         config.IsModifiedReviewer(name, def),
		}
		if isBuiltin && def.Description == preset.Description {
			info.Description = i18n.T(def.Description)
		}
		if len(def.Command) > 0 {
			_, err := review.LookupAgent(def.Command[0])
			info.Installed = err == nil
		}
		if health, ok := healthByName[name]; ok {
			info.Health = &health
			info.HealthLabel = review.HealthLabel(health.Status)
		}
		reviewers = append(reviewers, info)
	}
	return reviewers
}

// localizedTemplates are the third-party starting points for "Add reviewer".
func localizedTemplates() []config.ReviewerTemplate {
	templates := make([]config.ReviewerTemplate, len(config.ReviewerTemplates))
	for index, template := range config.ReviewerTemplates {
		template.Description = i18n.T(template.Description)
		template.Note = i18n.T(template.Note)
		templates[index] = template
	}
	return templates
}

// shortDuration prints 20m instead of 20m0s.
func shortDuration(value time.Duration) string {
	text := value.String()
	if strings.HasSuffix(text, "m0s") {
		text = strings.TrimSuffix(text, "0s")
	}
	if strings.HasSuffix(text, "h0m") {
		text = strings.TrimSuffix(text, "0m")
	}
	return text
}

func (s *Server) findReviewerInfo(cfg *config.Config, name string) *reviewerInfo {
	for _, info := range s.reviewerInfos(cfg) {
		if info.Name == name {
			return &info
		}
	}
	return nil
}

// handlePutReviewer adds a custom reviewer or changes an existing one.
func (s *Server) handlePutReviewer(w http.ResponseWriter, r *http.Request) {
	var def config.ReviewerDef
	if err := decodeBody(r, &def); err != nil {
		s.writeError(w, err)
		return
	}
	name := r.PathValue("name")
	updated, err := s.app.SaveReviewer(name, def)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviewer": s.findReviewerInfo(updated, name), "reviewers": s.reviewerInfos(updated)})
}

// handleDeleteReviewer deletes a custom reviewer or resets a built-in one.
func (s *Server) handleDeleteReviewer(w http.ResponseWriter, r *http.Request) {
	updated, err := s.app.RemoveReviewer(r.PathValue("name"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviewers": s.reviewerInfos(updated)})
}

// handleTestReviewer runs the smoke test for one reviewer (up to two minutes).
func (s *Server) handleTestReviewer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	health, err := s.app.Reviews.TestReviewer(r.Context(), name)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "health": health, "health_label": review.HealthLabel(health.Status)})
}

// handleTestReviewers smoke-tests every installed reviewer in parallel.
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
