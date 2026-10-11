package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// XteamTask is one slice in a project's .xteam/tasks archive.
type XteamTask struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Stage     string    `json:"stage"`
	Closed    bool      `json:"closed"`
	Verdict   string    `json:"verdict,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// XteamInfo summarizes xteam slice progress.
type XteamInfo struct {
	Open   int         `json:"open"`
	Closed int         `json:"closed"`
	Tasks  []XteamTask `json:"tasks"`
}

// ChangelogInfo is the newest entry of the project's changelog.
type ChangelogInfo struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Date    string `json:"date,omitempty"`
	Excerpt string `json:"excerpt"`
}

// xteamStages in protocol order; the furthest file present is the stage.
var xteamStages = []struct{ file, stage string }{
	{"spec.md", "spec"},
	{"request.md", "request"},
	{"delivered.json", "delivered"},
	{"ready.json", "ready"},
	{"verdict.json", "verdict"},
	{"consumed.json", "consumed"},
	{"closed.md", "closed"},
}

const maxXteamTasks = 50

// ReadXteam reads .xteam/tasks/* if present; nil otherwise.
func ReadXteam(root string) *XteamInfo {
	taskDirs, _ := filepath.Glob(filepath.Join(root, ".xteam", "tasks", "*"))
	if len(taskDirs) == 0 {
		return nil
	}
	info := &XteamInfo{Tasks: []XteamTask{}}
	for _, taskDir := range taskDirs {
		if stat, err := os.Stat(taskDir); err != nil || !stat.IsDir() {
			continue
		}
		task := readXteamTask(taskDir)
		if task.Closed {
			info.Closed++
		} else {
			info.Open++
		}
		info.Tasks = append(info.Tasks, task)
	}
	sort.Slice(info.Tasks, func(i, j int) bool { return info.Tasks[i].UpdatedAt.After(info.Tasks[j].UpdatedAt) })
	if len(info.Tasks) > maxXteamTasks {
		info.Tasks = info.Tasks[:maxXteamTasks]
	}
	return info
}

func readXteamTask(taskDir string) XteamTask {
	task := XteamTask{Slug: filepath.Base(taskDir), Stage: "new"}
	entries, _ := os.ReadDir(taskDir)
	for _, entry := range entries {
		if fileInfo, err := entry.Info(); err == nil && fileInfo.ModTime().After(task.UpdatedAt) {
			task.UpdatedAt = fileInfo.ModTime().UTC()
		}
	}
	for _, stage := range xteamStages {
		if _, err := os.Stat(filepath.Join(taskDir, stage.file)); err == nil {
			task.Stage = stage.stage
		}
	}
	task.Closed = task.Stage == "closed"
	if content, err := readCapped(filepath.Join(taskDir, "spec.md"), 8*1024); err == nil {
		task.Title = extractTitle(content, task.Slug)
	}
	if raw, err := os.ReadFile(filepath.Join(taskDir, "verdict.json")); err == nil {
		var verdict struct {
			Verdict string `json:"verdict"`
		}
		if json.Unmarshal(raw, &verdict) == nil {
			task.Verdict = verdict.Verdict
		}
	}
	return task
}

var changelogHeading = regexp.MustCompile(`^##\s+\[?v?(\d+\.\d+[0-9A-Za-z.\-+]*)\]?(?:.*?(\d{4}-\d{2}-\d{2}))?`)

// ReadChangelog parses the newest version section of the root changelog.
func ReadChangelog(root string, docs []Doc) *ChangelogInfo {
	candidate := ""
	for _, doc := range docs {
		if doc.Kind == "changelog" && !strings.Contains(doc.Path, "/") {
			candidate = doc.Path
			break
		}
	}
	if candidate == "" {
		return nil
	}
	content, err := readCapped(filepath.Join(root, candidate), 256*1024)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		match := changelogHeading.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		var excerpt []string
		for _, next := range lines[index+1:] {
			if strings.HasPrefix(next, "## ") {
				break
			}
			excerpt = append(excerpt, next)
		}
		text := strings.TrimSpace(strings.Join(excerpt, "\n"))
		if len(text) > 1500 {
			text = text[:1500] + "\n…"
		}
		return &ChangelogInfo{Path: candidate, Version: match[1], Date: match[2], Excerpt: text}
	}
	return &ChangelogInfo{Path: candidate}
}

// DetectStack infers the tech stack from marker files at the project root.
func DetectStack(root string) []string {
	stack := []string{}
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(root, name))
		return err == nil
	}
	fileContains := func(name, needle string) bool {
		content, err := readCapped(filepath.Join(root, name), 256*1024)
		return err == nil && strings.Contains(string(content), needle)
	}
	if exists("go.mod") {
		stack = append(stack, "Go")
	}
	if exists("composer.json") {
		stack = append(stack, "PHP")
		if fileContains("composer.json", `"laravel/framework"`) {
			stack = append(stack, "Laravel")
		}
		if fileContains("composer.json", `"filament/filament"`) {
			stack = append(stack, "Filament")
		}
	}
	if exists("package.json") {
		stack = append(stack, "Node")
		for needle, label := range map[string]string{`"react"`: "React", `"vue"`: "Vue", `"next"`: "Next.js", `"electron"`: "Electron"} {
			if fileContains("package.json", needle) {
				stack = append(stack, label)
			}
		}
	}
	if exists("bun.lockb") || exists("bun.lock") || exists("bunfig.toml") {
		stack = append(stack, "Bun")
	}
	if xcode, _ := filepath.Glob(filepath.Join(root, "*.xcodeproj")); len(xcode) > 0 || exists("Package.swift") {
		stack = append(stack, "Swift")
	}
	if exists("pyproject.toml") || exists("requirements.txt") || exists("setup.py") {
		stack = append(stack, "Python")
	}
	if exists("Cargo.toml") {
		stack = append(stack, "Rust")
	}
	if exists("project.config.json") || exists("miniprogram") {
		stack = append(stack, "Mini Program")
	}
	if exists("Dockerfile") || exists("docker-compose.yml") || exists("compose.yaml") {
		stack = append(stack, "Docker")
	}
	if exists(".github/workflows") {
		stack = append(stack, "GitHub Actions")
	}
	if exists(".xteam") {
		stack = append(stack, "xteam")
	}
	sort.Strings(stack)
	return stack
}
