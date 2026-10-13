package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/config"
)

// ErrReviewerInUse means a custom reviewer cannot be deleted while the global
// default or a project still points at it.
var ErrReviewerInUse = errors.New("reviewer is in use")

// SaveReviewer adds a custom reviewer or changes an existing one. Changing a
// reviewer forgets its health, so "auto" tests it again before relying on it.
func (a *App) SaveReviewer(name string, def config.ReviewerDef) (*config.Config, error) {
	def = trimReviewer(def)
	if preset, isBuiltin := config.BuiltinReviewers()[name]; isBuiltin {
		def = fillFromPreset(def, preset)
	}
	if err := config.ValidateReviewer(name, def); err != nil {
		return nil, err
	}
	updated, err := a.UpdateConfig(func(cfg *config.Config) error {
		cfg.Reviewers[name] = def
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Reviews.ForgetReviewerHealth(name)
	return updated, nil
}

// RemoveReviewer deletes a custom reviewer, or resets a built-in preset to
// what xplan ships.
func (a *App) RemoveReviewer(name string) (*config.Config, error) {
	builtin, isBuiltin := config.BuiltinReviewers()[name]
	if !isBuiltin {
		if err := a.ensureReviewerUnused(name); err != nil {
			return nil, err
		}
	}
	updated, err := a.UpdateConfig(func(cfg *config.Config) error {
		if _, ok := cfg.Reviewers[name]; !ok {
			return fmt.Errorf("%w: reviewer %q is not configured", config.ErrReviewer, name)
		}
		if isBuiltin {
			cfg.Reviewers[name] = builtin
			return nil
		}
		delete(cfg.Reviewers, name)
		cfg.ReviewerOrder = slices.DeleteFunc(cfg.ReviewerOrder, func(item string) bool { return item == name })
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.Reviews.ForgetReviewerHealth(name)
	return updated, nil
}

// ensureReviewerUnused lists who still uses a reviewer that is about to go.
func (a *App) ensureReviewerUnused(name string) error {
	users := []string{}
	if a.Config().DefaultReviewer == name {
		users = append(users, "default_reviewer")
	}
	projectList, err := a.Store.List()
	if err != nil {
		return err
	}
	for _, item := range projectList {
		if item.Reviewer == name {
			users = append(users, item.Name)
		}
	}
	if len(users) > 0 {
		return fmt.Errorf("%w: %s is used by %s; pick another reviewer there first", ErrReviewerInUse, name, strings.Join(users, ", "))
	}
	return nil
}

// fillFromPreset treats empty fields of a built-in as "the preset's value",
// so clearing opencode's agent brings back "plan" instead of its writing
// default agent.
func fillFromPreset(def, preset config.ReviewerDef) config.ReviewerDef {
	if def.Description == "" {
		def.Description = preset.Description
	}
	if len(def.Command) == 0 {
		def.Command = preset.Command
	}
	if def.Model == "" {
		def.Model = preset.Model
	}
	if def.Agent == "" {
		def.Agent = preset.Agent
	}
	if def.Timeout == "" {
		def.Timeout = preset.Timeout
	}
	return def
}

func trimReviewer(def config.ReviewerDef) config.ReviewerDef {
	command := make([]string, 0, len(def.Command))
	for _, part := range def.Command {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			command = append(command, trimmed)
		}
	}
	return config.ReviewerDef{
		Description: strings.TrimSpace(def.Description),
		Command:     command,
		Model:       strings.TrimSpace(def.Model),
		Agent:       strings.TrimSpace(def.Agent),
		Timeout:     strings.TrimSpace(def.Timeout),
	}
}
