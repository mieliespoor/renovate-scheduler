package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeRepoStore struct {
	mu             sync.Mutex
	syncCalls      [][]string
	claimCalls     []claimCall
	completeCalls  []completeCall
	claimDueResult []string
	claimDueErr    error
}

type claimCall struct {
	now         time.Time
	minInterval time.Duration
	limit       int
}

type completeCall struct {
	repo   string
	runErr error
}

func (f *fakeRepoStore) SyncRepos(repos []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncCalls = append(f.syncCalls, repos)
	return nil
}

func (f *fakeRepoStore) ClaimDue(now time.Time, minInterval time.Duration, limit int) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimCalls = append(f.claimCalls, claimCall{now, minInterval, limit})
	if f.claimDueErr != nil {
		return nil, f.claimDueErr
	}
	return f.claimDueResult, nil
}

func (f *fakeRepoStore) Complete(repo string, runErr error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls = append(f.completeCalls, completeCall{repo, runErr})
	return nil
}

func (f *fakeRepoStore) getSyncCalls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.syncCalls
}

func (f *fakeRepoStore) getClaimCalls() []claimCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.claimCalls
}

func (f *fakeRepoStore) getCompleteCalls() []completeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.completeCalls
}

func TestIngestLoopReadsAndSyncsRepos(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.json")

	// Write initial repos
	if err := os.WriteFile(reposPath, []byte(`["org/repo-a", "org/repo-b"]`), 0o644); err != nil {
		t.Fatalf("write repos: %v", err)
	}

	cfg := &Config{
		Scheduler: SchedulerConfig{
			ReposPollSeconds: 1,
		},
	}
	store := &fakeRepoStore{}

	// Run ingest loop for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = ingestLoop(ctx, reposPath, cfg, store)

	syncCalls := store.getSyncCalls()
	if len(syncCalls) == 0 {
		t.Fatalf("expected at least one sync call")
	}
	if len(syncCalls[0]) != 2 {
		t.Fatalf("expected 2 repos in first sync, got %d", len(syncCalls[0]))
	}
}

func TestIngestLoopDetectsFileChanges(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.json")

	if err := os.WriteFile(reposPath, []byte(`["org/repo-a"]`), 0o644); err != nil {
		t.Fatalf("write repos: %v", err)
	}

	cfg := &Config{
		Scheduler: SchedulerConfig{
			ReposPollSeconds: 1,
		},
	}
	store := &fakeRepoStore{}

	ctx, cancel := context.WithCancel(context.Background())
	writeErrCh := make(chan error, 1)
	go func() {
		time.Sleep(500 * time.Millisecond)
		// Modify repos file
		if err := os.WriteFile(reposPath, []byte(`["org/repo-a", "org/repo-b", "org/repo-c"]`), 0o644); err != nil {
			writeErrCh <- err
			cancel()
			return
		}
		writeErrCh <- nil
		time.Sleep(1500 * time.Millisecond)
		cancel()
	}()

	_ = ingestLoop(ctx, reposPath, cfg, store)

	if writeErr := <-writeErrCh; writeErr != nil {
		t.Fatalf("update repos: %v", writeErr)
	}

	syncCalls := store.getSyncCalls()
	if len(syncCalls) < 2 {
		t.Logf("sync calls: %v", syncCalls)
		t.Fatalf("expected at least 2 sync calls (initial + file change), got %d", len(syncCalls))
	}
	if len(syncCalls[0]) != 1 {
		t.Fatalf("first sync: expected 1 repo, got %d", len(syncCalls[0]))
	}
	if len(syncCalls[1]) != 3 {
		t.Fatalf("second sync: expected 3 repos, got %d", len(syncCalls[1]))
	}
}

func TestDispatchLoopClaimsAndProcessesRepos(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{
			Namespace: "default",
		},
		Renovate: RenovateConfig{
			Image: "renovate/renovate:latest",
		},
		Scheduler: SchedulerConfig{
			MaxConcurrentTasks:  2,
			RunIntervalMinutes:  60,
			DispatchPollSeconds: 1,
		},
	}

	store := &fakeRepoStore{
		claimDueResult: []string{"org/repo-a", "org/repo-b"},
	}

	processedRepos := make([]string, 0)
	mu := sync.Mutex{}

	// Patch runRenovateJob to track calls
	originalFunc := runRenovateJob
	defer func() { runRenovateJob = originalFunc }()

	runRenovateJob = func(ctx context.Context, client interface{}, cfg *Config, repo string) error {
		// client is nil in test, so we don't use it
		mu.Lock()
		processedRepos = append(processedRepos, repo)
		mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = dispatchLoop(ctx, nil, cfg, store)

	mu.Lock()
	if len(processedRepos) == 0 {
		t.Fatalf("expected repos to be processed")
	}
	mu.Unlock()

	claimCalls := store.getClaimCalls()
	if len(claimCalls) == 0 {
		t.Fatalf("expected ClaimDue to be called")
	}

	completeCalls := store.getCompleteCalls()
	if len(completeCalls) == 0 {
		t.Fatalf("expected Complete to be called")
	}
}

func TestDispatchLoopRespectsMaxConcurrency(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{
			Namespace: "default",
		},
		Renovate: RenovateConfig{
			Image: "renovate/renovate:latest",
		},
		Scheduler: SchedulerConfig{
			MaxConcurrentTasks:  1,
			RunIntervalMinutes:  60,
			DispatchPollSeconds: 1,
		},
	}

	store := &fakeRepoStore{
		claimDueResult: []string{"org/repo-a"},
	}

	concurrent := 0
	maxConcurrent := 0
	mu := sync.Mutex{}

	originalFunc := runRenovateJob
	defer func() { runRenovateJob = originalFunc }()

	runRenovateJob = func(ctx context.Context, client interface{}, cfg *Config, repo string) error {
		// client is nil in test, so we don't use it
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		concurrent--
		mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	_ = dispatchLoop(ctx, nil, cfg, store)

	if maxConcurrent > 1 {
		t.Fatalf("expected max concurrent 1, got %d", maxConcurrent)
	}
}

func TestDispatchLoopHandlesClaimErrors(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{
			Namespace: "default",
		},
		Renovate: RenovateConfig{
			Image: "renovate/renovate:latest",
		},
		Scheduler: SchedulerConfig{
			MaxConcurrentTasks:  1,
			RunIntervalMinutes:  60,
			DispatchPollSeconds: 1,
		},
	}

	store := &fakeRepoStore{
		claimDueErr: fmt.Errorf("claim failed"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should not crash despite error
	_ = dispatchLoop(ctx, nil, cfg, store)

	// Verify ClaimDue was still called despite error
	if len(store.getClaimCalls()) == 0 {
		t.Fatalf("expected ClaimDue to be called")
	}
}

func TestDispatchLoopStopsOnContextCancel(t *testing.T) {
	cfg := &Config{
		Kubernetes: KubernetesConfig{
			Namespace: "default",
		},
		Renovate: RenovateConfig{
			Image: "renovate/renovate:latest",
		},
		Scheduler: SchedulerConfig{
			MaxConcurrentTasks:  1,
			RunIntervalMinutes:  60,
			DispatchPollSeconds: 1,
		},
	}

	store := &fakeRepoStore{}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_ = dispatchLoop(ctx, nil, cfg, store)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Fatalf("dispatchLoop should have exited quickly after cancel, took %v", elapsed)
	}
}

func TestIngestLoopStopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	reposPath := filepath.Join(dir, "repos.json")

	if err := os.WriteFile(reposPath, []byte(`["org/repo-a"]`), 0o644); err != nil {
		t.Fatalf("write repos: %v", err)
	}

	cfg := &Config{
		Scheduler: SchedulerConfig{
			ReposPollSeconds: 1,
		},
	}
	store := &fakeRepoStore{}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_ = ingestLoop(ctx, reposPath, cfg, store)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Fatalf("ingestLoop should have exited quickly after cancel, took %v", elapsed)
	}
}
