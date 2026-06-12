package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeRepos(t *testing.T) {
	in := []string{" org/repo-b ", "org/repo-a", "", "org/repo-b", "  "}
	got := normalizeRepos(in)
	want := []string{"org/repo-a", "org/repo-b"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeRepos()=%v, want %v", got, want)
	}
}

func TestLoadRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")
	content := `[
  "org/repo-b",
  " org/repo-a ",
  "org/repo-b",
  ""
]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write repos file: %v", err)
	}

	got, err := loadRepos(path)
	if err != nil {
		t.Fatalf("loadRepos: %v", err)
	}
	want := []string{"org/repo-a", "org/repo-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadRepos()=%v, want %v", got, want)
	}
}

func TestLoadReposErrors(t *testing.T) {
	if _, err := loadRepos(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("expected error for missing repos file")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}
	if _, err := loadRepos(path); err == nil {
		t.Fatalf("expected parse error for invalid json")
	}
}
