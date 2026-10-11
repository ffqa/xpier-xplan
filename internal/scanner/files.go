package scanner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// ErrOutsideRoot is returned when a requested path escapes the project root.
var ErrOutsideRoot = errors.New("path is outside the project")

// ErrSensitive is returned for files that are never served (secrets, keys).
var ErrSensitive = errors.New("sensitive file is hidden")

// ErrNotText is returned for binary or oversized files.
var ErrNotText = errors.New("not a readable text file")

// maxFileBytes is the largest file the browser will return.
const maxFileBytes = 1024 * 1024

// Entry is one item in a directory listing.
type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size"`
	Sensitive bool   `json:"sensitive"`
}

// ResolveInside joins relPath to root and guarantees the result (after
// resolving symlinks) stays within root.
func ResolveInside(root, relPath string) (string, error) {
	cleaned := filepath.Clean("/" + filepath.FromSlash(relPath))
	joined := filepath.Join(root, cleaned)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", os.ErrNotExist
	}
	if realPath != realRoot && !strings.HasPrefix(realPath, realRoot+string(os.PathSeparator)) {
		return "", ErrOutsideRoot
	}
	return realPath, nil
}

// IsSensitive reports whether a path looks like it holds secrets.
func IsSensitive(relPath string) bool {
	lower := strings.ToLower(filepath.ToSlash(relPath))
	base := filepath.Base(lower)
	if strings.HasPrefix(base, ".env") && base != ".env.example" && base != ".env.sample" {
		return true
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx", ".keystore", ".jks", ".mobileprovision"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	if strings.HasPrefix(base, "id_rsa") || strings.HasPrefix(base, "id_ed25519") || base == "credentials.json" {
		return true
	}
	for _, segment := range strings.Split(lower, "/") {
		if segment == "secrets" || segment == ".ssh" || segment == "keys" {
			return true
		}
	}
	return false
}

// ListDir lists one directory level, directories first, hiding heavy dirs.
func ListDir(root, relPath string) ([]Entry, error) {
	dir, err := ResolveInside(root, relPath)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	realRoot, _ := filepath.EvalSymlinks(root)
	entries := []Entry{}
	for _, item := range items {
		if item.Name() == ".git" || item.Name() == ".DS_Store" || (item.IsDir() && skipDirs[item.Name()]) {
			continue
		}
		fullPath := filepath.Join(dir, item.Name())
		rel, _ := filepath.Rel(realRoot, fullPath)
		rel = filepath.ToSlash(rel)
		entry := Entry{Name: item.Name(), Path: rel, IsDir: item.IsDir(), Sensitive: IsSensitive(rel)}
		if info, err := item.Info(); err == nil && !item.IsDir() {
			entry.Size = info.Size()
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// ReadText returns the content of a text file inside root, refusing
// sensitive, binary and oversized files.
func ReadText(root, relPath string) (string, error) {
	if IsSensitive(relPath) {
		return "", ErrSensitive
	}
	fullPath, err := ResolveInside(root, relPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", os.ErrNotExist
	}
	if info.IsDir() || info.Size() > maxFileBytes {
		return "", ErrNotText
	}
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if !utf8.Valid(content) || strings.ContainsRune(string(content[:min(len(content), 8000)]), 0) {
		return "", ErrNotText
	}
	return string(content), nil
}
