package scanner

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Doc is one collected document inside the project.
type Doc struct {
	Path      string    `json:"path"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	Hash      string    `json:"hash"`
	Checked   int       `json:"checked"`
	Unchecked int       `json:"unchecked"`
}

// Doc kinds, in the order the UI groups them.
var DocKinds = []string{"readme", "spec", "adr", "progress", "changelog", "agent", "xteam", "doc"}

const (
	maxDocs       = 400
	maxDepth      = 5
	maxDocBytes   = 512 * 1024
	maxXteamSpecs = 60
)

// skipDirs are never descended into: dependencies, build output, vendored code.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "out": true,
	"target": true, "coverage": true, "Pods": true, "DerivedData": true,
	"third_party": true, "third-party": true, "Frameworks": true, "librime": true, "plum": true,
	"venv": true, "__pycache__": true, "storage": true, "bootstrap": true,
	"public": true, "tmp": true, "temp": true, "cache": true, "logs": true, "Sparkle": true,
}

var docExtensions = map[string]bool{".md": true, ".mdx": true, ".markdown": true}

var checkboxPattern = regexp.MustCompile(`(?m)^\s*(?:[-*+]|\d+\.)\s+\[([ xX])\]`)

// CollectDocs walks the project for Markdown documents and classifies them.
// It returns the docs, whether the cap was hit, and non-fatal errors.
func CollectDocs(root string, extraGlobs []string) ([]Doc, bool, []string) {
	docs := []Doc{}
	seen := map[string]bool{}
	errorsSeen := []string{}
	truncated := false

	addDoc := func(relPath string) {
		if seen[relPath] || truncated {
			return
		}
		if len(docs) >= maxDocs {
			truncated = true
			return
		}
		doc, err := readDoc(root, relPath)
		if err != nil {
			return
		}
		seen[relPath] = true
		docs = append(docs, doc)
	}

	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip quietly
		}
		relPath, _ := filepath.Rel(root, path)
		if entry.IsDir() {
			if relPath == "." {
				return nil
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") || skipDirs[name] || strings.Count(relPath, string(os.PathSeparator)) >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if isDocFile(entry.Name()) {
			addDoc(filepath.ToSlash(relPath))
		}
		return nil
	})
	if walkErr != nil {
		errorsSeen = append(errorsSeen, "walk docs: "+walkErr.Error())
	}

	for _, relPath := range xteamDocPaths(root) {
		addDoc(relPath)
	}
	for _, pattern := range extraGlobs {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			errorsSeen = append(errorsSeen, "bad doc glob "+pattern)
			continue
		}
		for _, match := range matches {
			if relPath, err := filepath.Rel(root, match); err == nil && !strings.HasPrefix(relPath, "..") {
				addDoc(filepath.ToSlash(relPath))
			}
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		if kindRank(docs[i].Kind) != kindRank(docs[j].Kind) {
			return kindRank(docs[i].Kind) < kindRank(docs[j].Kind)
		}
		return docs[i].Path < docs[j].Path
	})
	return docs, truncated, errorsSeen
}

func isDocFile(name string) bool {
	if docExtensions[strings.ToLower(filepath.Ext(name))] {
		return true
	}
	upper := strings.ToUpper(name)
	return upper == "README" || upper == "CHANGELOG"
}

// xteamDocPaths returns the queue and the most recently touched slice specs
// from .xteam (hidden dirs are otherwise skipped).
func xteamDocPaths(root string) []string {
	paths := []string{}
	if _, err := os.Stat(filepath.Join(root, ".xteam", "QUEUE.md")); err == nil {
		paths = append(paths, ".xteam/QUEUE.md")
	}
	specs, _ := filepath.Glob(filepath.Join(root, ".xteam", "tasks", "*", "spec.md"))
	sort.Slice(specs, func(i, j int) bool { return modTime(specs[i]).After(modTime(specs[j])) })
	if len(specs) > maxXteamSpecs {
		specs = specs[:maxXteamSpecs]
	}
	for _, spec := range specs {
		if relPath, err := filepath.Rel(root, spec); err == nil {
			paths = append(paths, filepath.ToSlash(relPath))
		}
	}
	return paths
}

func readDoc(root, relPath string) (Doc, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return Doc{}, os.ErrNotExist
	}
	content, err := readCapped(fullPath, maxDocBytes)
	if err != nil {
		return Doc{}, err
	}
	sum := sha1.Sum(content)
	checked, unchecked := countCheckboxes(content)
	return Doc{
		Path:      relPath,
		Kind:      ClassifyDoc(relPath),
		Title:     extractTitle(content, filepath.Base(relPath)),
		Size:      info.Size(),
		ModTime:   info.ModTime().UTC(),
		Hash:      hex.EncodeToString(sum[:])[:12],
		Checked:   checked,
		Unchecked: unchecked,
	}, nil
}

var (
	adrNamePattern = regexp.MustCompile(`^\d{3,4}[-_]`)
	specWords      = []string{"spec", "prd", "rfc", "requirement", "需求", "规格", "设计", "design", "architecture", "契约", "contract"}
	progressWords  = []string{"progress", "todo", "roadmap", "plan", "status", "handoff", "milestone", "baseline", "进度", "计划", "交接", "路线", "里程碑"}
	agentNames     = map[string]bool{"agents.md": true, "claude.md": true, "gemini.md": true, "skill.md": true, "copilot-instructions.md": true}
)

// ClassifyDoc infers a document kind from its relative path.
func ClassifyDoc(relPath string) string {
	lower := strings.ToLower(relPath)
	base := filepath.Base(lower)
	segments := strings.Split(filepath.Dir(lower), "/")
	hasSegment := func(names ...string) bool {
		for _, segment := range segments {
			for _, name := range names {
				if segment == name {
					return true
				}
			}
		}
		return false
	}
	switch {
	case strings.HasPrefix(lower, ".xteam/"):
		return "xteam"
	case strings.HasPrefix(base, "readme"):
		return "readme"
	case strings.HasPrefix(base, "changelog") || strings.HasPrefix(base, "changes") ||
		strings.HasPrefix(base, "history") || strings.HasPrefix(base, "release"):
		return "changelog"
	case agentNames[base]:
		return "agent"
	case hasSegment("adr", "adrs", "decisions") || strings.HasPrefix(base, "adr") ||
		(hasSegment("docs", "doc") && adrNamePattern.MatchString(base) && strings.Contains(lower, "decision")):
		return "adr"
	case hasSegment("spec", "specs", "prd", "rfc", "rfcs") || containsAny(base, specWords):
		return "spec"
	case containsAny(base, progressWords):
		return "progress"
	default:
		return "doc"
	}
}

func containsAny(text string, words []string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func kindRank(kind string) int {
	for index, candidate := range DocKinds {
		if candidate == kind {
			return index
		}
	}
	return len(DocKinds)
}

func extractTitle(content []byte, fallback string) string {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for lineNumber := 0; scanner.Scan() && lineNumber < 40; lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return fallback
}

func countCheckboxes(content []byte) (int, int) {
	checked, unchecked := 0, 0
	for _, match := range checkboxPattern.FindAllSubmatch(content, -1) {
		if string(match[1]) == " " {
			unchecked++
		} else {
			checked++
		}
	}
	return checked, unchecked
}

func readCapped(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, limit)
	read, err := file.Read(buffer)
	if err != nil && read == 0 {
		return []byte{}, nil
	}
	return buffer[:read], nil
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
