package main

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestNewJSONRepoStoreRequiresPath(t *testing.T) {
	if _, err := newJSONRepoStore(""); err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestJSONRepoStoreSyncClaimCompleteFlow(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state", "scheduler-state.json")
	store, err := newJSONRepoStore(statePath)
	if err != nil {
		t.Fatalf("newJSONRepoStore: %v", err)
	}

	if err := store.SyncRepos([]string{"org/repo-a", "org/repo-b"}); err != nil {
		t.Fatalf("SyncRepos: %v", err)
	}

	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	due, err := store.ClaimDue(now, 60*time.Minute, 1)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected exactly one claimed repo, got %d (%v)", len(due), due)
	}
	first := due[0]

	st := store.state.Repos[first]
	if !st.InProgress {
		t.Fatalf("claimed repo should be in progress")
	}
	if !st.LastStarted.Equal(now) {
		t.Fatalf("LastStarted=%v, want %v", st.LastStarted, now)
	}

	runErr := errors.New("job failed")
	if err := store.Complete(first, runErr); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if store.state.Repos[first].InProgress {
		t.Fatalf("repo should not be in progress after Complete")
	}
	if store.state.Repos[first].LastError != runErr.Error() {
		t.Fatalf("LastError=%q, want %q", store.state.Repos[first].LastError, runErr.Error())
	}

	if err := store.Complete(first, nil); err != nil {
		t.Fatalf("Complete success: %v", err)
	}
	if store.state.Repos[first].LastError != "" {
		t.Fatalf("LastError should be cleared on success")
	}
}

func TestJSONRepoStoreClaimDueHonorsIntervalAndInProgress(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "scheduler-state.json")
	store, err := newJSONRepoStore(statePath)
	if err != nil {
		t.Fatalf("newJSONRepoStore: %v", err)
	}

	if err := store.SyncRepos([]string{"a/repo", "b/repo"}); err != nil {
		t.Fatalf("SyncRepos: %v", err)
	}

	now := time.Date(2026, 6, 12, 11, 0, 0, 0, time.UTC)
	claimed, err := store.ClaimDue(now, 60*time.Minute, 2)
	if err != nil {
		t.Fatalf("ClaimDue first: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected both repos claimed, got %d", len(claimed))
	}

	claimedAgain, err := store.ClaimDue(now.Add(30*time.Minute), 60*time.Minute, 2)
	if err != nil {
		t.Fatalf("ClaimDue second: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("expected no repos due within interval, got %v", claimedAgain)
	}

	for _, repo := range claimed {
		if err := store.Complete(repo, nil); err != nil {
			t.Fatalf("Complete(%s): %v", repo, err)
		}
	}

	claimedAfterInterval, err := store.ClaimDue(now.Add(61*time.Minute), 60*time.Minute, 2)
	if err != nil {
		t.Fatalf("ClaimDue after interval: %v", err)
	}
	if len(claimedAfterInterval) != 2 {
		t.Fatalf("expected both repos due after interval, got %v", claimedAfterInterval)
	}
}

func TestJSONRepoStoreSyncDisablesMissingRepos(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "scheduler-state.json")
	store, err := newJSONRepoStore(statePath)
	if err != nil {
		t.Fatalf("newJSONRepoStore: %v", err)
	}

	if err := store.SyncRepos([]string{"org/repo-a", "org/repo-b"}); err != nil {
		t.Fatalf("SyncRepos first: %v", err)
	}
	if err := store.SyncRepos([]string{"org/repo-b"}); err != nil {
		t.Fatalf("SyncRepos second: %v", err)
	}

	a := store.state.Repos["org/repo-a"]
	b := store.state.Repos["org/repo-b"]
	if a == nil || b == nil {
		t.Fatalf("expected both repos to exist in state")
	}
	if a.Enabled {
		t.Fatalf("org/repo-a should be disabled after being removed from input")
	}
	if !b.Enabled {
		t.Fatalf("org/repo-b should remain enabled")
	}
}

func TestJSONRepoStorePersistsAndReloads(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "scheduler-state.json")
	store, err := newJSONRepoStore(statePath)
	if err != nil {
		t.Fatalf("newJSONRepoStore: %v", err)
	}

	if err := store.SyncRepos([]string{"org/repo-a"}); err != nil {
		t.Fatalf("SyncRepos: %v", err)
	}

	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	claimed, err := store.ClaimDue(now, 60*time.Minute, 1)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one claimed repo")
	}
	if err := store.Complete(claimed[0], nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	reloaded, err := newJSONRepoStore(statePath)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	state := reloaded.state.Repos["org/repo-a"]
	if state == nil {
		t.Fatalf("expected repo in reloaded state")
	}
	if state.LastFinished.IsZero() {
		t.Fatalf("expected LastFinished to persist")
	}
	if state.InProgress {
		t.Fatalf("InProgress should be false after complete")
	}
}
