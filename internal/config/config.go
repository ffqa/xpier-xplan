// Package config loads and persists xplan's global configuration
// (~/.xplan/config.yaml) and resolves the home directory.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// ReviewerDef describes how to invoke one agent CLI in a read-only mode.
// Command is an argv template; the placeholders {prompt} and {path} are
// substituted at run time. No shell is involved.
type ReviewerDef struct {
	Description string   `yaml:"description" json:"description"`
	Command     []string `yaml:"command" json:"command"`
}

// Config is the global configuration stored in <home>/config.yaml.
type Config struct {
	Listen               string                 `yaml:"listen" json:"listen"`
	ScanInterval         string                 `yaml:"scan_interval" json:"scan_interval"`
	ReviewTimeout        string                 `yaml:"review_timeout" json:"review_timeout"`
	MaxConcurrentReviews int                    `yaml:"max_concurrent_reviews" json:"max_concurrent_reviews"`
	DefaultReviewer      string                 `yaml:"default_reviewer" json:"default_reviewer"`
	ReportLanguage       string                 `yaml:"report_language" json:"report_language"`
	DefaultSchedule      map[string]string      `yaml:"default_schedule" json:"default_schedule"`
	Reviewers            map[string]ReviewerDef `yaml:"reviewers" json:"reviewers"`
}

// FileName is the config file name inside the home directory.
const FileName = "config.yaml"

// ResolveHome returns the xplan home: explicit value, $XPLAN_HOME, or ~/.xplan.
func ResolveHome(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	if env := os.Getenv("XPLAN_HOME"); env != "" {
		return filepath.Abs(env)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(userHome, ".xplan"), nil
}

// Defaults returns the built-in configuration. Every reviewer preset uses the
// agent's own read-only mode.
func Defaults() Config {
	return Config{
		Listen:               "127.0.0.1:7788",
		ScanInterval:         "15m",
		ReviewTimeout:        "20m",
		MaxConcurrentReviews: 2,
		DefaultReviewer:      "cursor-agent",
		ReportLanguage:       "English",
		DefaultSchedule: map[string]string{
			"drift":    "weekly",
			"security": "weekly",
			"code":     "off",
			"deploy":   "off",
			"test":     "off",
		},
		Reviewers: map[string]ReviewerDef{
			"cursor-agent": {
				Description: "Cursor Agent in ask mode (read-only Q&A)",
				Command:     []string{"cursor-agent", "-p", "--output-format", "text", "--mode", "ask", "--trust", "--workspace", "{path}", "{prompt}"},
			},
			"claude": {
				Description: "Claude Code print mode limited to Read/Grep/Glob",
				// The tool flags are variadic: "=" keeps them from swallowing the prompt.
				Command: []string{"claude", "-p", "{prompt}", "--allowedTools=Read,Grep,Glob", "--disallowedTools=Edit,Write,Bash,NotebookEdit"},
			},
			"codex": {
				Description: "Codex exec in read-only sandbox",
				Command:     []string{"codex", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "-C", "{path}", "{prompt}"},
			},
			"omp": {
				Description: "omp print mode with read-only tools",
				Command:     []string{"omp", "-p", "--no-session", "--cwd", "{path}", "--tools", "read,grep,glob", "{prompt}"},
			},
		},
	}
}

// Load reads <home>/config.yaml, creating it with defaults on first run.
// Missing fields fall back to defaults so older files keep working.
func Load(home string) (*Config, error) {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, fmt.Errorf("create home %s: %w", home, err)
	}
	path := filepath.Join(home, FileName)
	cfg := Defaults()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := Save(home, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var fromFile Config
	if err := yaml.Unmarshal(raw, &fromFile); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	merge(&cfg, &fromFile)
	return &cfg, nil
}

// merge overlays non-zero values from src onto dst.
func merge(dst, src *Config) {
	if src.Listen != "" {
		dst.Listen = src.Listen
	}
	if src.ScanInterval != "" {
		dst.ScanInterval = src.ScanInterval
	}
	if src.ReviewTimeout != "" {
		dst.ReviewTimeout = src.ReviewTimeout
	}
	if src.MaxConcurrentReviews > 0 {
		dst.MaxConcurrentReviews = src.MaxConcurrentReviews
	}
	if src.DefaultReviewer != "" {
		dst.DefaultReviewer = src.DefaultReviewer
	}
	if src.ReportLanguage != "" {
		dst.ReportLanguage = src.ReportLanguage
	}
	for kind, schedule := range src.DefaultSchedule {
		dst.DefaultSchedule[kind] = schedule
	}
	for name, def := range src.Reviewers {
		if isLegacyPreset(name, def.Command) {
			continue // keep the corrected built-in preset
		}
		dst.Reviewers[name] = def
	}
}

// legacyPresets lists built-in commands that earlier versions wrote into
// config.yaml and that are known to be broken.
var legacyPresets = map[string][][]string{
	// --disallowedTools is variadic and swallowed the prompt.
	"claude": {{"claude", "-p", "--allowedTools", "Read,Grep,Glob", "--disallowedTools", "Edit,Write,Bash,NotebookEdit", "{prompt}"}},
	// omp has no "ls" tool and rejects the whole list.
	"omp": {{"omp", "-p", "--no-session", "--cwd", "{path}", "--tools", "read,grep,find,ls", "{prompt}"}},
}

func isLegacyPreset(name string, command []string) bool {
	for _, legacy := range legacyPresets[name] {
		if slices.Equal(legacy, command) {
			return true
		}
	}
	return false
}

// Save writes the config atomically.
func Save(home string, cfg *Config) error {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return WriteFileAtomic(filepath.Join(home, FileName), raw)
}

// ScanEvery parses ScanInterval, falling back to 15 minutes.
func (c *Config) ScanEvery() time.Duration {
	return parseDurationOr(c.ScanInterval, 15*time.Minute)
}

// ReviewDeadline parses ReviewTimeout, falling back to 20 minutes.
func (c *Config) ReviewDeadline() time.Duration {
	return parseDurationOr(c.ReviewTimeout, 20*time.Minute)
}

// ReviewerNames returns the configured reviewer names in stable order.
func (c *Config) ReviewerNames() []string {
	names := make([]string, 0, len(c.Reviewers))
	for name := range c.Reviewers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseDurationOr(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// WriteFileAtomic writes to a temp file in the same dir and renames it, so a
// crash never leaves a half-written file behind.
func WriteFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", path, err)
	}
	return nil
}
