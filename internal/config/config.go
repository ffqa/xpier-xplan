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

// Config is the global configuration stored in <home>/config.yaml.
type Config struct {
	Listen               string                 `yaml:"listen" json:"listen"`
	ScanInterval         string                 `yaml:"scan_interval" json:"scan_interval"`
	ReviewTimeout        string                 `yaml:"review_timeout" json:"review_timeout"`
	MaxConcurrentReviews int                    `yaml:"max_concurrent_reviews" json:"max_concurrent_reviews"`
	DefaultReviewer      string                 `yaml:"default_reviewer" json:"default_reviewer"`
	ReviewerOrder        []string               `yaml:"reviewer_order" json:"reviewer_order"`
	Language             string                 `yaml:"language" json:"language"`
	ReportLanguage       string                 `yaml:"report_language" json:"report_language"`
	DefaultSchedule      map[string]string      `yaml:"default_schedule" json:"default_schedule"`
	Reviewers            map[string]ReviewerDef `yaml:"reviewers" json:"reviewers"`
}

// FileName is the config file name inside the home directory.
const FileName = "config.yaml"

// AutoReviewer is the reviewer value that picks the first working preset in
// ReviewerOrder at run time.
const AutoReviewer = "auto"

// Languages the UI and server messages support.
var Languages = []string{"en", "zh"}

// KimiAgentFile is the read-only agent profile xplan writes for kimi, relative
// to the xplan home: kimi's -p mode auto-approves every tool it has, so the
// profile is what limits it to Read/Grep/Glob.
const KimiAgentFile = "agents/kimi-readonly.md"

// KimiAgentProfile is the content of KimiAgentFile.
const KimiAgentProfile = `---
name: xplan-reviewer
description: Read-only external reviewer started by xplan. It may read the project but never change it.
tools: [Read, Grep, Glob]
subagents: []
---
${base_prompt}
`

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
		DefaultReviewer:      AutoReviewer,
		ReviewerOrder:        []string{"cursor-agent", "codex", "claude", "kimi", "gemini", "qwen", "opencode", "kiro", "devin", "omp"},
		Language:             "en",
		ReportLanguage:       "English",
		DefaultSchedule: map[string]string{
			"drift":    "weekly",
			"security": "weekly",
			"code":     "off",
			"deploy":   "off",
			"test":     "off",
		},
		Reviewers: BuiltinReviewers(),
	}
}

// IsLanguage reports whether value is a supported UI language.
func IsLanguage(value string) bool {
	return slices.Contains(Languages, value)
}

// EnsureAgentFiles writes the helper files some presets point at.
func EnsureAgentFiles(home string) error {
	return WriteFileAtomic(filepath.Join(home, KimiAgentFile), []byte(KimiAgentProfile))
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
	if len(src.ReviewerOrder) > 0 {
		dst.ReviewerOrder = src.ReviewerOrder
	}
	if IsLanguage(src.Language) {
		dst.Language = src.Language
	}
	if src.ReportLanguage != "" {
		dst.ReportLanguage = src.ReportLanguage
	}
	for kind, schedule := range src.DefaultSchedule {
		dst.DefaultSchedule[kind] = schedule
	}
	for name, def := range src.Reviewers {
		if preset, isBuiltin := dst.Reviewers[name]; isBuiltin {
			dst.Reviewers[name] = overlayReviewer(preset, def, name)
			continue
		}
		if len(def.Command) > 0 {
			dst.Reviewers[name] = def
		}
	}
}

// Save writes the config atomically. Built-in reviewers are stored only as
// the fields that differ from the preset.
func Save(home string, cfg *Config) error {
	stored := *cfg
	stored.Reviewers = storedReviewers(cfg.Reviewers)
	raw, err := yaml.Marshal(&stored)
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

// ReviewDeadlineFor is the reviewer's own timeout, or the global one.
func (c *Config) ReviewDeadlineFor(name string) time.Duration {
	return parseDurationOr(c.Reviewers[name].Timeout, c.ReviewDeadline())
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

// AutoCandidates returns the reviewers "auto" may pick, in order: the
// configured ReviewerOrder first, then every other preset alphabetically, so
// presets added later are never left out.
func (c *Config) AutoCandidates() []string {
	candidates := []string{}
	seen := map[string]bool{}
	for _, name := range c.ReviewerOrder {
		if _, ok := c.Reviewers[name]; ok && !seen[name] {
			candidates = append(candidates, name)
			seen[name] = true
		}
	}
	for _, name := range c.ReviewerNames() {
		if !seen[name] {
			candidates = append(candidates, name)
		}
	}
	return candidates
}

// IsReviewer reports whether name is a configured preset or "auto".
func (c *Config) IsReviewer(name string) bool {
	if name == AutoReviewer {
		return true
	}
	_, ok := c.Reviewers[name]
	return ok
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
