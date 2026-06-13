package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "config.toml", "path to the TOML configuration file")
	reposPath := flag.String("repos", "renovate-repos.json", "path to the JSON file listing repositories")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("renovate-scheduler %s\n", Version)
		os.Exit(0)
	}

	if err := run(*configPath, *reposPath); err != nil {
		appLogger.Error("renovate-scheduler failed", "error", err)
		os.Exit(1)
	}
}

func run(configPath, reposPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	if err := configureLogger(cfg.Logging.Format); err != nil {
		return fmt.Errorf("configuring logger: %w", err)
	}

	client, err := newClient(cfg)
	if err != nil {
		return fmt.Errorf("connecting to kubernetes: %w", err)
	}

	store, err := newJSONRepoStore(cfg.Scheduler.StateFilePath)
	if err != nil {
		return fmt.Errorf("initializing state store: %w", err)
	}

	// Cancel in-flight work on the first interrupt signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appLogger.Info("starting scheduler",
		"max_concurrent_tasks", cfg.Scheduler.MaxConcurrentTasks,
		"run_interval", time.Duration(cfg.Scheduler.RunIntervalMinutes)*time.Minute,
		"namespace", cfg.Kubernetes.Namespace,
		"log_format", cfg.Logging.Format,
	)

	errCh := make(chan error, 2)
	go func() {
		errCh <- ingestLoop(ctx, reposPath, cfg, store)
	}()
	go func() {
		errCh <- dispatchLoop(ctx, client, cfg, store)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("interrupted: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return err
		}
		return nil
	}
}

func ingestLoop(ctx context.Context, reposPath string, cfg *Config, store RepoStore) error {
	pollEvery := time.Duration(cfg.Scheduler.ReposPollSeconds) * time.Second
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()

	var previousDigest string

	for {
		repos, err := loadRepos(reposPath)
		if err != nil {
			appLogger.Error("ingest failed to load repositories", "error", err)
		} else {
			normalized := normalizeRepos(repos)
			digest := strings.Join(normalized, "\n")
			if digest != previousDigest {
				if err := store.SyncRepos(normalized); err != nil {
					appLogger.Error("ingest failed to sync repositories", "error", err)
				} else {
					previousDigest = digest
					appLogger.Info("ingest synced repositories", "tracked_repositories", len(normalized))
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func dispatchLoop(ctx context.Context, client kubeClient, cfg *Config, store RepoStore) error {
	interval := time.Duration(cfg.Scheduler.RunIntervalMinutes) * time.Minute
	pollEvery := time.Duration(cfg.Scheduler.DispatchPollSeconds) * time.Second
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()

	sem := make(chan struct{}, cfg.Scheduler.MaxConcurrentTasks)
	var wg sync.WaitGroup

	for {
		available := cap(sem) - len(sem)
		if available > 0 {
			dueRepos, err := store.ClaimDue(time.Now(), interval, available)
			if err != nil {
				appLogger.Error("dispatch failed to claim due repositories", "error", err)
			} else {
				for _, repo := range dueRepos {
					sem <- struct{}{}
					wg.Add(1)
					go func(repo string) {
						defer wg.Done()
						defer func() { <-sem }()

						start := time.Now()
						appLogger.Info("dispatch started repository run", "repository", repo)
						err := runRenovateJob(ctx, client, cfg, repo)
						duration := time.Since(start)

						if err != nil {
							appLogger.Error("dispatch repository run failed",
								"repository", repo,
								"duration", duration.Round(time.Second),
								"error", err,
							)
						} else {
							appLogger.Info("dispatch repository run completed",
								"repository", repo,
								"duration", duration.Round(time.Second),
							)
						}

						if completeErr := store.Complete(repo, err); completeErr != nil {
							appLogger.Error("dispatch failed to complete repository state",
								"repository", repo,
								"error", completeErr,
							)
						}
					}(repo)
				}
			}
		}

		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-ticker.C:
		}
	}
}

// loadRepos read a JSON array of repository paths from path.
func loadRepos(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading repos file %q: %w", path, err)
	}

	var repos []string
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("parsing repos file %q: %w", path, err)
	}
	return normalizeRepos(repos), nil
}

func normalizeRepos(repos []string) []string {
	unique := make(map[string]struct{}, len(repos))
	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			continue
		}
		unique[repo] = struct{}{}
	}

	out := make([]string, 0, len(unique))
	for repo := range unique {
		out = append(out, repo)
	}
	sort.Strings(out)
	return out
}
