package mcp

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ffqa/xpier-xplan/internal/project"
)

// Target names a project either by slug or by a path inside it. Tools embed
// it so agents can simply pass their working directory.
type Target struct {
	Project string `json:"project"`
	Path    string `json:"path"`
}

var targetProperties = map[string]any{
	"path": map[string]any{
		"type":        "string",
		"description": "Absolute path of your working directory (or any path inside the project). Preferred.",
	},
	"project": map[string]any{
		"type":        "string",
		"description": "Project slug from xplan_projects, when you are not working inside the project.",
	},
}

// resolve finds the project for a target. A path matches the deepest
// registered project that contains it, so nested repositories resolve to
// themselves rather than to their parent.
func (h *Handler) resolve(target Target) (*project.Project, error) {
	slug := strings.TrimSpace(target.Project)
	path := strings.TrimSpace(target.Path)
	switch {
	case slug != "":
		return h.bySlug(slug)
	case path != "":
		return h.byPath(path)
	default:
		return nil, errors.New("pass `path` (your working directory) or `project` (a slug from xplan_projects)")
	}
}

func (h *Handler) bySlug(slug string) (*project.Project, error) {
	found, err := h.app.Store.Get(slug)
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, project.ErrNotFound) {
		return nil, err
	}
	projectList, listErr := h.app.Store.List()
	if listErr != nil {
		return nil, listErr
	}
	for _, item := range projectList {
		if strings.EqualFold(item.Name, slug) {
			return item, nil
		}
	}
	return nil, fmt.Errorf("no xplan project is called %q; registered: %s", slug, slugList(projectList))
}

func (h *Handler) byPath(raw string) (*project.Project, error) {
	if !filepath.IsAbs(raw) {
		return nil, fmt.Errorf("path must be absolute, got %q", raw)
	}
	wanted := canonicalPath(raw)
	projectList, err := h.app.Store.List()
	if err != nil {
		return nil, err
	}
	var best *project.Project
	bestLength := -1
	for _, item := range projectList {
		root := canonicalPath(item.Path)
		if pathWithin(root, wanted) && len(root) > bestLength {
			best, bestLength = item, len(root)
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no xplan project contains %s; register it in xplan first (registered: %s)", raw, slugList(projectList))
	}
	return best, nil
}

// canonicalPath resolves symlinks when the path exists (macOS /tmp is
// /private/tmp) and otherwise just cleans it.
func canonicalPath(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved
	}
	return cleaned
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func slugList(projectList []*project.Project) string {
	if len(projectList) == 0 {
		return "none"
	}
	slugs := make([]string, 0, len(projectList))
	for _, item := range projectList {
		slugs = append(slugs, item.Slug)
	}
	return strings.Join(slugs, ", ")
}
