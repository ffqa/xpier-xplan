// Package project manages the registry of projects tracked by xplan and the
// files xplan owns for each of them (intent, ADRs, journal, state).
// It never writes into the project directory itself.
package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
	"gopkg.in/yaml.v3"
)

// Statuses a project can be in; they double as the board columns.
var Statuses = []string{"active", "waiting", "blocked", "paused", "done"}

// ReviewKinds that can be scheduled.
var ScheduledKinds = []string{"drift", "code", "security", "deploy", "test"}

// ScheduleValues allowed per review kind.
var ScheduleValues = []string{"off", "daily", "weekly"}

// ErrNotFound is returned when a slug is not registered.
var ErrNotFound = errors.New("project not found")

// ErrInvalid wraps validation failures so the HTTP layer can map them to 400.
var ErrInvalid = errors.New("invalid input")

// Project is the user-editable configuration stored in project.yaml.
type Project struct {
	Slug      string            `yaml:"-" json:"slug"`
	Name      string            `yaml:"name" json:"name"`
	Path      string            `yaml:"path" json:"path"`
	Status    string            `yaml:"status" json:"status"`
	Summary   string            `yaml:"summary" json:"summary"`
	Tags      []string          `yaml:"tags" json:"tags"`
	Reviewer  string            `yaml:"reviewer" json:"reviewer"`
	Schedule  map[string]string `yaml:"schedule" json:"schedule"`
	DocGlobs  []string          `yaml:"doc_globs" json:"doc_globs"`
	CreatedAt time.Time         `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time         `yaml:"updated_at" json:"updated_at"`
}

// Patch carries optional field updates; nil means "leave unchanged".
type Patch struct {
	Name     *string           `json:"name"`
	Status   *string           `json:"status"`
	Summary  *string           `json:"summary"`
	Tags     []string          `json:"tags"`
	Reviewer *string           `json:"reviewer"`
	Schedule map[string]string `json:"schedule"`
	DocGlobs []string          `json:"doc_globs"`
}

// Store is the file-backed registry rooted at <home>/projects.
type Store struct {
	home            string
	defaultSchedule map[string]string
	mu              sync.Mutex
}

// NewStore creates a store; defaultSchedule seeds new projects.
func NewStore(home string, defaultSchedule map[string]string) *Store {
	return &Store{home: home, defaultSchedule: defaultSchedule}
}

// Home returns the xplan home directory.
func (s *Store) Home() string { return s.home }

// Dir returns the xplan-owned directory of a project.
func (s *Store) Dir(slug string) string {
	return filepath.Join(s.home, "projects", slug)
}

// List returns all registered projects sorted by name.
func (s *Store) List() ([]*Project, error) {
	root := filepath.Join(s.home, "projects")
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []*Project{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projectList := make([]*Project, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		loaded, err := s.Get(entry.Name())
		if err != nil {
			continue // a broken folder must not hide every other project
		}
		projectList = append(projectList, loaded)
	}
	sort.Slice(projectList, func(i, j int) bool {
		return strings.ToLower(projectList[i].Name) < strings.ToLower(projectList[j].Name)
	})
	return projectList, nil
}

// Get loads one project by slug.
func (s *Store) Get(slug string) (*Project, error) {
	if !validSlug(slug) {
		return nil, ErrNotFound
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir(slug), "project.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read project %s: %w", slug, err)
	}
	var loaded Project
	if err := yaml.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("parse project %s: %w", slug, err)
	}
	loaded.Slug = slug
	if loaded.Schedule == nil {
		loaded.Schedule = map[string]string{}
	}
	return &loaded, nil
}

// Add registers a directory. It seeds intent.md from the project's README.
func (s *Store) Add(path, name string) (*Project, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(expandHome(path)))
	if err != nil || path == "" {
		return nil, fmt.Errorf("%w: path is required", ErrInvalid)
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrInvalid, absPath)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, other := range existing {
		if other.Path == absPath {
			return nil, fmt.Errorf("%w: already registered as %q", ErrInvalid, other.Slug)
		}
	}

	slug := s.uniqueSlug(slugify(filepath.Base(absPath)))
	if name == "" {
		name = filepath.Base(absPath)
	}
	now := time.Now().UTC()
	readmeExcerpt := ReadmeExcerpt(absPath)
	created := &Project{
		Slug:      slug,
		Name:      name,
		Path:      absPath,
		Status:    "active",
		Summary:   firstSentence(readmeExcerpt, 160),
		Tags:      []string{},
		Schedule:  copySchedule(s.defaultSchedule),
		DocGlobs:  []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.save(created); err != nil {
		return nil, err
	}
	if err := s.WriteIntent(slug, IntentTemplate(name, readmeExcerpt)); err != nil {
		return nil, err
	}
	return created, nil
}

// Update applies a patch after validating every field.
func (s *Store) Update(slug string, patch Patch) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.Get(slug)
	if err != nil {
		return nil, err
	}
	if patch.Name != nil {
		trimmed := strings.TrimSpace(*patch.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: name cannot be empty", ErrInvalid)
		}
		current.Name = trimmed
	}
	if patch.Status != nil {
		if !contains(Statuses, *patch.Status) {
			return nil, fmt.Errorf("%w: status must be one of %v", ErrInvalid, Statuses)
		}
		current.Status = *patch.Status
	}
	if patch.Summary != nil {
		current.Summary = strings.TrimSpace(*patch.Summary)
	}
	if patch.Tags != nil {
		current.Tags = cleanList(patch.Tags)
	}
	if patch.Reviewer != nil {
		current.Reviewer = strings.TrimSpace(*patch.Reviewer)
	}
	for kind, value := range patch.Schedule {
		if !contains(ScheduledKinds, kind) || !contains(ScheduleValues, value) {
			return nil, fmt.Errorf("%w: schedule %s=%s not allowed", ErrInvalid, kind, value)
		}
		current.Schedule[kind] = value
	}
	if patch.DocGlobs != nil {
		current.DocGlobs = cleanList(patch.DocGlobs)
	}
	current.UpdatedAt = time.Now().UTC()
	if err := s.save(current); err != nil {
		return nil, err
	}
	return current, nil
}

// Remove unregisters a project by moving its xplan folder into archive/.
// Nothing is deleted, and the project directory itself is never touched.
func (s *Store) Remove(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.Get(slug); err != nil {
		return err
	}
	archiveDir := filepath.Join(s.home, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	target := filepath.Join(archiveDir, slug+"-"+time.Now().Format("20060102-150405"))
	if err := os.Rename(s.Dir(slug), target); err != nil {
		return fmt.Errorf("archive %s: %w", slug, err)
	}
	return nil
}

func (s *Store) save(p *Project) error {
	raw, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode project %s: %w", p.Slug, err)
	}
	return config.WriteFileAtomic(filepath.Join(s.Dir(p.Slug), "project.yaml"), raw)
}

func (s *Store) uniqueSlug(base string) string {
	if base == "" {
		base = "project"
	}
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(s.Dir(candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	return strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

var validSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validSlug(slug string) bool { return validSlugPattern.MatchString(slug) }

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if userHome, err := os.UserHomeDir(); err == nil {
			return filepath.Join(userHome, path[2:])
		}
	}
	return path
}

func copySchedule(source map[string]string) map[string]string {
	result := map[string]string{}
	for _, kind := range ScheduledKinds {
		result[kind] = "off"
	}
	for kind, value := range source {
		if contains(ScheduledKinds, kind) && contains(ScheduleValues, value) {
			result[kind] = value
		}
	}
	return result
}

func cleanList(values []string) []string {
	cleaned := []string{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
