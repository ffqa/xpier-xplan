// Package scanner collects read-only facts about a project directory: git
// activity, the documents the project writes about itself, xteam slice
// progress, checklists, changelog and tech stack.
package scanner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ffqa/xplan/internal/config"
)

// Snapshot is the result of one scan.
type Snapshot struct {
	ScannedAt     time.Time      `json:"scanned_at"`
	DurationMs    int64          `json:"duration_ms"`
	PathExists    bool           `json:"path_exists"`
	Git           *GitInfo       `json:"git,omitempty"`
	Docs          []Doc          `json:"docs"`
	DocsTruncated bool           `json:"docs_truncated"`
	Xteam         *XteamInfo     `json:"xteam,omitempty"`
	Checklist     Checklist      `json:"checklist"`
	Changelog     *ChangelogInfo `json:"changelog,omitempty"`
	Stack         []string       `json:"stack"`
	Errors        []string       `json:"errors,omitempty"`
}

// Checklist aggregates Markdown task boxes across collected docs.
type Checklist struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// historyKeep is how many historical scan files are retained per project.
const historyKeep = 30

// Scan collects a snapshot for the directory at projectPath. extraGlobs are
// additional doc patterns (relative to the project root) configured by the user.
func Scan(projectPath string, extraGlobs []string) *Snapshot {
	started := time.Now()
	snapshot := &Snapshot{ScannedAt: started.UTC(), Docs: []Doc{}, Stack: []string{}}

	info, err := os.Stat(projectPath)
	if err != nil || !info.IsDir() {
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("project path not found: %s", projectPath))
		snapshot.DurationMs = time.Since(started).Milliseconds()
		return snapshot
	}
	snapshot.PathExists = true

	gitInfo, gitErr := ReadGit(projectPath)
	if gitErr != nil {
		snapshot.Errors = append(snapshot.Errors, "git: "+gitErr.Error())
	}
	snapshot.Git = gitInfo

	docs, truncated, docErrors := CollectDocs(projectPath, extraGlobs)
	snapshot.Docs = docs
	snapshot.DocsTruncated = truncated
	snapshot.Errors = append(snapshot.Errors, docErrors...)
	for _, doc := range docs {
		snapshot.Checklist.Done += doc.Checked
		snapshot.Checklist.Total += doc.Checked + doc.Unchecked
	}

	snapshot.Changelog = ReadChangelog(projectPath, docs)
	snapshot.Xteam = ReadXteam(projectPath)
	snapshot.Stack = DetectStack(projectPath)
	snapshot.DurationMs = time.Since(started).Milliseconds()
	return snapshot
}

// Save writes scans/latest.json plus a timestamped history copy, rotating old
// history files.
func Save(projectDir string, snapshot *Snapshot) error {
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	scansDir := filepath.Join(projectDir, "scans")
	if err := config.WriteFileAtomic(filepath.Join(scansDir, "latest.json"), raw); err != nil {
		return err
	}
	historyName := snapshot.ScannedAt.Format("20060102-150405") + ".json"
	if err := config.WriteFileAtomic(filepath.Join(scansDir, historyName), raw); err != nil {
		return err
	}
	return rotateHistory(scansDir)
}

// LoadLatest reads scans/latest.json; returns (nil, nil) when never scanned.
func LoadLatest(projectDir string) (*Snapshot, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "scans", "latest.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read latest scan: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("parse latest scan: %w", err)
	}
	return &snapshot, nil
}

func rotateHistory(scansDir string) error {
	entries, err := os.ReadDir(scansDir)
	if err != nil {
		return fmt.Errorf("list scan history: %w", err)
	}
	history := []string{}
	for _, entry := range entries {
		if entry.Name() != "latest.json" && filepath.Ext(entry.Name()) == ".json" {
			history = append(history, entry.Name())
		}
	}
	sort.Strings(history)
	for len(history) > historyKeep {
		if err := os.Remove(filepath.Join(scansDir, history[0])); err != nil {
			return fmt.Errorf("rotate scan history: %w", err)
		}
		history = history[1:]
	}
	return nil
}
