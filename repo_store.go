package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type RepoStore interface {
	SyncRepos(repos []string) error
	ClaimDue(now time.Time, minInterval time.Duration, limit int) ([]string, error)
	Complete(repo string, runErr error) error
}

type RepoState struct {
	Repo         string    `json:"repo"`
	Enabled      bool      `json:"enabled"`
	InProgress   bool      `json:"in_progress"`
	LastStarted  time.Time `json:"last_started,omitempty"`
	LastFinished time.Time `json:"last_finished,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type repoStateFile struct {
	Version int                   `json:"version"`
	Repos   map[string]*RepoState `json:"repos"`
}

type JSONRepoStore struct {
	path  string
	mu    sync.Mutex
	state repoStateFile
}

func newJSONRepoStore(path string) (*JSONRepoStore, error) {
	if path == "" {
		return nil, fmt.Errorf("state file path must be set")
	}

	s := &JSONRepoStore{
		path: path,
		state: repoStateFile{
			Version: 1,
			Repos:   map[string]*RepoState{},
		},
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	if err := s.persist(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *JSONRepoStore) SyncRepos(repos []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	seen := make(map[string]struct{}, len(repos))

	for _, repo := range repos {
		seen[repo] = struct{}{}
		current, ok := s.state.Repos[repo]
		if !ok {
			s.state.Repos[repo] = &RepoState{
				Repo:      repo,
				Enabled:   true,
				UpdatedAt: now,
			}
			continue
		}
		current.Enabled = true
		current.UpdatedAt = now
	}

	for repo, current := range s.state.Repos {
		if _, ok := seen[repo]; ok {
			continue
		}
		current.Enabled = false
		current.UpdatedAt = now
	}

	return s.persistLocked()
}

func (s *JSONRepoStore) ClaimDue(now time.Time, minInterval time.Duration, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	candidates := make([]string, 0, len(s.state.Repos))
	for repo, st := range s.state.Repos {
		if !st.Enabled || st.InProgress {
			continue
		}
		if !st.LastStarted.IsZero() && now.Sub(st.LastStarted) < minInterval {
			continue
		}
		candidates = append(candidates, repo)
	}
	sort.Strings(candidates)

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	for _, repo := range candidates {
		st := s.state.Repos[repo]
		st.InProgress = true
		st.LastStarted = now
		st.UpdatedAt = now
	}

	if len(candidates) == 0 {
		return nil, nil
	}
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (s *JSONRepoStore) Complete(repo string, runErr error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.state.Repos[repo]
	if !ok {
		return nil
	}

	now := time.Now().UTC()
	st.InProgress = false
	st.LastFinished = now
	st.UpdatedAt = now
	if runErr != nil {
		st.LastError = runErr.Error()
	} else {
		st.LastError = ""
	}

	return s.persistLocked()
}

func (s *JSONRepoStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading state file %q: %w", s.path, err)
	}

	var state repoStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parsing state file %q: %w", s.path, err)
	}
	if state.Repos == nil {
		state.Repos = map[string]*RepoState{}
	}
	if state.Version == 0 {
		state.Version = 1
	}
	s.state = state
	return nil
}

func (s *JSONRepoStore) persist() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked()
}

func (s *JSONRepoStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding state file: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing state file temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replacing state file: %w", err)
	}
	return nil
}
