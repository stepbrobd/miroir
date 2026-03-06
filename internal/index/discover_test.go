package index

import (
	"os"
	"path/filepath"
	"testing"
)

func initBareRepo(t *testing.T, path string) {
	t.Helper()
	for _, d := range []string{"objects", "refs"} {
		if err := os.MkdirAll(filepath.Join(path, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initNonBareRepo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverBareRepos(t *testing.T) {
	dir := t.TempDir()
	initBareRepo(t, filepath.Join(dir, "repo-a.git"))
	initBareRepo(t, filepath.Join(dir, "repo-b.git"))

	got, err := Discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d repos, want 2", len(got))
	}
}

func TestDiscoverNonBareRepos(t *testing.T) {
	dir := t.TempDir()
	initNonBareRepo(t, filepath.Join(dir, "repo-a"))

	got, err := Discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d repos, want 1", len(got))
	}
}

func TestDiscoverMixed(t *testing.T) {
	dir := t.TempDir()
	initBareRepo(t, filepath.Join(dir, "bare.git"))
	initNonBareRepo(t, filepath.Join(dir, "normal"))
	os.MkdirAll(filepath.Join(dir, "notrepo"), 0o755)

	got, err := Discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d repos, want 2: %v", len(got), got)
	}
}

func TestDiscoverNoRecurse(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "org", "repo.git")
	initBareRepo(t, nested)

	got, err := Discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d repos, want 0 (no recursion): %v", len(got), got)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	got, err := Discover(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d repos, want 0", len(got))
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := Discover([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d repos, want 0", len(got))
	}
}

func TestDiscoverMissingDir(t *testing.T) {
	_, err := Discover([]string{"/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}
