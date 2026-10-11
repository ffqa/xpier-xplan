package scanner

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Commit is a single git commit summary.
type Commit struct {
	Hash    string    `json:"hash"`
	Short   string    `json:"short"`
	At      time.Time `json:"at"`
	Author  string    `json:"author"`
	Subject string    `json:"subject"`
}

// GitInfo summarizes repository activity.
type GitInfo struct {
	Branch       string     `json:"branch"`
	Head         string     `json:"head"`
	LastCommitAt *time.Time `json:"last_commit_at,omitempty"`
	DirtyFiles   int        `json:"dirty_files"`
	DirtySample  []string   `json:"dirty_sample"`
	Commits7d    int        `json:"commits_7d"`
	Commits30d   int        `json:"commits_30d"`
	Remote       string     `json:"remote,omitempty"`
	Recent       []Commit   `json:"recent"`
}

const gitTimeout = 15 * time.Second

// runGit executes git read-only subcommands inside dir with a timeout.
func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(dir string) bool {
	out, err := runGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// ReadGit gathers repository facts. Returns (nil, nil) for non-repos.
func ReadGit(dir string) (*GitInfo, error) {
	if !IsRepo(dir) {
		return nil, nil
	}
	info := &GitInfo{DirtySample: []string{}, Recent: []Commit{}}

	if branch, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = branch
	}
	head, err := runGit(dir, "rev-parse", "HEAD")
	if err != nil {
		// Fresh repository without commits: still report dirty state.
		info.DirtyFiles, info.DirtySample = dirtyFiles(dir)
		return info, nil
	}
	info.Head = head
	if remote, err := runGit(dir, "remote", "get-url", "origin"); err == nil {
		info.Remote = remote
	}
	info.Recent, err = Log(dir, "", 20)
	if err != nil {
		return info, err
	}
	if len(info.Recent) > 0 {
		at := info.Recent[0].At
		info.LastCommitAt = &at
	}
	info.Commits7d = countSince(dir, "7.days")
	info.Commits30d = countSince(dir, "30.days")
	info.DirtyFiles, info.DirtySample = dirtyFiles(dir)
	return info, nil
}

const logFormat = "--pretty=format:%H%x1f%ct%x1f%an%x1f%s"

// Log returns up to limit commits for the given revision range ("" = HEAD).
func Log(dir, revRange string, limit int) ([]Commit, error) {
	args := []string{"log", "-n", strconv.Itoa(limit), logFormat}
	if revRange != "" {
		args = append(args, revRange)
	}
	out, err := runGit(dir, args...)
	if err != nil {
		return []Commit{}, err
	}
	return parseLog(out), nil
}

// CommitsSince returns commits after a previously seen HEAD. When that commit
// is not an ancestor any more (rebase, reset), it falls back to time.
func CommitsSince(dir, sinceHead string, since time.Time, limit int) ([]Commit, error) {
	if sinceHead != "" && isAncestor(dir, sinceHead) {
		return Log(dir, sinceHead+"..HEAD", limit)
	}
	out, err := runGit(dir, "log", "-n", strconv.Itoa(limit), logFormat, "--since="+since.Format(time.RFC3339))
	if err != nil {
		return []Commit{}, err
	}
	return parseLog(out), nil
}

// ChangedFilesSince lists paths changed between sinceHead and HEAD plus
// uncommitted changes in the working tree.
func ChangedFilesSince(dir, sinceHead string) []string {
	seen := map[string]bool{}
	changed := []string{}
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			changed = append(changed, path)
		}
	}
	if sinceHead != "" && isAncestor(dir, sinceHead) {
		if out, err := runGit(dir, "diff", "--name-only", sinceHead, "HEAD"); err == nil {
			for _, line := range strings.Split(out, "\n") {
				add(strings.TrimSpace(line))
			}
		}
	}
	_, dirty := dirtyFilesAll(dir)
	for _, path := range dirty {
		add(path)
	}
	return changed
}

// StatusFingerprint hashes `git status --porcelain` so callers can detect
// whether the working tree changed between two points in time.
func StatusFingerprint(dir string) string {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil {
		return ""
	}
	sum := sha1.Sum([]byte(out))
	return hex.EncodeToString(sum[:])
}

func isAncestor(dir, commit string) bool {
	_, err := runGit(dir, "merge-base", "--is-ancestor", commit, "HEAD")
	return err == nil
}

func countSince(dir, since string) int {
	out, err := runGit(dir, "rev-list", "--count", "--since="+since, "HEAD")
	if err != nil {
		return 0
	}
	count, _ := strconv.Atoi(strings.TrimSpace(out))
	return count
}

func dirtyFiles(dir string) (int, []string) {
	count, paths := dirtyFilesAll(dir)
	if len(paths) > 15 {
		paths = paths[:15]
	}
	return count, paths
}

func dirtyFilesAll(dir string) (int, []string) {
	out, err := runGit(dir, "status", "--porcelain")
	if err != nil || out == "" {
		return 0, []string{}
	}
	lines := strings.Split(out, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 3 {
			path := strings.TrimSpace(line[3:])
			if arrow := strings.Index(path, " -> "); arrow >= 0 {
				path = path[arrow+4:]
			}
			paths = append(paths, strings.Trim(path, `"`))
		}
	}
	return len(paths), paths
}

func parseLog(out string) []Commit {
	commits := []Commit{}
	if strings.TrimSpace(out) == "" {
		return commits
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) != 4 {
			continue
		}
		unix, _ := strconv.ParseInt(parts[1], 10, 64)
		short := parts[0]
		if len(short) > 7 {
			short = short[:7]
		}
		commits = append(commits, Commit{
			Hash:    parts[0],
			Short:   short,
			At:      time.Unix(unix, 0).UTC(),
			Author:  parts[2],
			Subject: parts[3],
		})
	}
	return commits
}
