package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ffqa/xpier-xplan/internal/config"
)

// Journal entry kinds.
const (
	EntryPark    = "park"
	EntryNote    = "note"
	EntryCommand = "command"
	EntrySystem  = "system"
)

// Entry is one line in journal.jsonl. A park note captures where you stopped
// so that coming back later does not depend on memory.
type Entry struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Text    string    `json:"text,omitempty"`
	Stopped string    `json:"stopped,omitempty"`
	Next    string    `json:"next,omitempty"`
	Open    string    `json:"open,omitempty"`
	Head    string    `json:"head,omitempty"`
}

// Visit records when you last looked at a project and at which commit.
type Visit struct {
	At   time.Time `json:"at"`
	Head string    `json:"head"`
}

// State is machine-maintained per-project state (kept out of project.yaml so
// that the user config stays quiet under git).
type State struct {
	LastVisit  *Visit     `json:"last_visit,omitempty"`
	LastScanAt *time.Time `json:"last_scan_at,omitempty"`
}

// AppendJournal validates and appends an entry, returning it with ID/time set.
func (s *Store) AppendJournal(slug string, entry Entry) (Entry, error) {
	if _, err := s.Get(slug); err != nil {
		return Entry{}, err
	}
	switch entry.Kind {
	case EntryPark:
		if strings.TrimSpace(entry.Stopped+entry.Next+entry.Open+entry.Text) == "" {
			return Entry{}, fmt.Errorf("%w: a park note needs at least one field", ErrInvalid)
		}
	case EntryNote, EntryCommand, EntrySystem:
		if strings.TrimSpace(entry.Text) == "" {
			return Entry{}, fmt.Errorf("%w: text is required", ErrInvalid)
		}
	default:
		return Entry{}, fmt.Errorf("%w: unknown entry kind %q", ErrInvalid, entry.Kind)
	}
	entry.At = time.Now().UTC()
	entry.ID = entry.At.Format("20060102T150405.000000")

	s.mu.Lock()
	defer s.mu.Unlock()
	line, err := json.Marshal(entry)
	if err != nil {
		return Entry{}, fmt.Errorf("encode journal entry: %w", err)
	}
	path := filepath.Join(s.Dir(slug), "journal.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Entry{}, fmt.Errorf("open journal %s: %w", slug, err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return Entry{}, fmt.Errorf("append journal %s: %w", slug, err)
	}
	return entry, nil
}

// ReadJournal returns entries newest first; kind filters when non-empty.
func (s *Store) ReadJournal(slug, kind string, limit int) ([]Entry, error) {
	if _, err := s.Get(slug); err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(s.Dir(slug), "journal.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open journal %s: %w", slug, err)
	}
	defer file.Close()

	all := []Entry{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // skip a corrupted line rather than losing the journal
		}
		if kind == "" || entry.Kind == kind {
			all = append(all, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read journal %s: %w", slug, err)
	}
	newestFirst := make([]Entry, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		newestFirst = append(newestFirst, all[i])
		if limit > 0 && len(newestFirst) >= limit {
			break
		}
	}
	return newestFirst, nil
}

// LastPark returns the most recent park note, or nil.
func (s *Store) LastPark(slug string) (*Entry, error) {
	entries, err := s.ReadJournal(slug, EntryPark, 1)
	if err != nil || len(entries) == 0 {
		return nil, err
	}
	return &entries[0], nil
}

// ReadState loads state.json (zero value if missing).
func (s *Store) ReadState(slug string) (State, error) {
	raw, err := os.ReadFile(filepath.Join(s.Dir(slug), "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state %s: %w", slug, err)
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, fmt.Errorf("parse state %s: %w", slug, err)
	}
	return state, nil
}

// UpdateState applies mutate to state.json under the store lock.
func (s *Store) UpdateState(slug string, mutate func(*State)) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.ReadState(slug)
	if err != nil {
		return State{}, err
	}
	mutate(&state)
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return State{}, fmt.Errorf("encode state %s: %w", slug, err)
	}
	if err := config.WriteFileAtomic(filepath.Join(s.Dir(slug), "state.json"), raw); err != nil {
		return State{}, err
	}
	return state, nil
}

// MarkVisit records "I'm caught up" at the given HEAD.
func (s *Store) MarkVisit(slug, head string) (State, error) {
	if _, err := s.Get(slug); err != nil {
		return State{}, err
	}
	return s.UpdateState(slug, func(state *State) {
		state.LastVisit = &Visit{At: time.Now().UTC(), Head: head}
	})
}
