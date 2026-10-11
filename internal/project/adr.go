package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ffqa/xplan/internal/config"
)

// ADR is one architecture/product decision record owned by xplan.
type ADR struct {
	ID        string    `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

var adrFilePattern = regexp.MustCompile(`^(\d{4})-[a-z0-9-]+\.md$`)
var adrStatusPattern = regexp.MustCompile(`(?mi)^\s*(?:-\s*)?\*{0,2}status\*{0,2}\s*:\s*\*{0,2}\s*([a-z-]+)`)

// ListADRs returns ADRs sorted by number.
func (s *Store) ListADRs(slug string) ([]ADR, error) {
	if _, err := s.Get(slug); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.Dir(slug), "adr")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []ADR{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list adrs %s: %w", slug, err)
	}
	adrList := []ADR{}
	for _, entry := range entries {
		match := adrFilePattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		loaded, err := readADR(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		adrList = append(adrList, loaded)
	}
	sort.Slice(adrList, func(i, j int) bool { return adrList[i].Number < adrList[j].Number })
	return adrList, nil
}

// CreateADR writes a new ADR with the next number and the standard template.
func (s *Store) CreateADR(slug, title, context string) (ADR, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return ADR{}, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	existing, err := s.ListADRs(slug)
	if err != nil {
		return ADR{}, err
	}
	next := 1
	if len(existing) > 0 {
		next = existing[len(existing)-1].Number + 1
	}
	fileSlug := slugify(title)
	if fileSlug == "" {
		fileSlug = "decision"
	}
	if len(fileSlug) > 48 {
		fileSlug = strings.Trim(fileSlug[:48], "-")
	}
	id := fmt.Sprintf("%04d-%s", next, fileSlug)
	content := fmt.Sprintf(
		"# %04d. %s\n\n- Status: proposed\n- Date: %s\n\n## Context\n\n%s\n\n## Decision\n\n\n## Consequences\n\n",
		next, title, time.Now().Format("2006-01-02"), strings.TrimSpace(context),
	)
	path := filepath.Join(s.Dir(slug), "adr", id+".md")
	if err := config.WriteFileAtomic(path, []byte(content)); err != nil {
		return ADR{}, err
	}
	return readADR(path)
}

// UpdateADR replaces the content of an existing ADR.
func (s *Store) UpdateADR(slug, id, content string) (ADR, error) {
	if _, err := s.Get(slug); err != nil {
		return ADR{}, err
	}
	if !adrFilePattern.MatchString(id + ".md") {
		return ADR{}, ErrNotFound
	}
	path := filepath.Join(s.Dir(slug), "adr", id+".md")
	if _, err := os.Stat(path); err != nil {
		return ADR{}, ErrNotFound
	}
	if err := config.WriteFileAtomic(path, []byte(content)); err != nil {
		return ADR{}, err
	}
	return readADR(path)
}

func readADR(path string) (ADR, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ADR{}, fmt.Errorf("read adr %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return ADR{}, fmt.Errorf("stat adr %s: %w", path, err)
	}
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	number, _ := strconv.Atoi(base[:4])
	content := string(raw)
	title := base
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}
	status := "proposed"
	if match := adrStatusPattern.FindStringSubmatch(content); match != nil {
		status = strings.ToLower(match[1])
	}
	return ADR{ID: base, Number: number, Title: title, Status: status, Content: content, UpdatedAt: info.ModTime()}, nil
}
