package index

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func skipNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
}

// creates a non-bare repo with one commit to serve as remote
func seedRepo(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "seed")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
	)
	for _, args := range [][]string{
		{"init", "--initial-branch=main"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %s", args, err, out)
		}
	}
	return src
}

func TestFetchCloneBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	path, err := Fetch(dest, r, true)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "test.git" {
		t.Errorf("path: got %q, want test.git suffix", path)
	}
	if !isBareRepo(path) {
		t.Error("expected bare repo")
	}
}

func TestFetchCloneNonBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	path, err := Fetch(dest, r, false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "test" {
		t.Errorf("path: got %q", path)
	}
	if !isNonBareRepo(path) {
		t.Error("expected non-bare repo")
	}
}

func TestFetchStatError(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()

	// create a non-readable dir so stat on children fails
	noread := filepath.Join(tmp, "noread")
	if err := os.MkdirAll(noread, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(noread, 0o000)
	t.Cleanup(func() { os.Chmod(noread, 0o755) })

	r := Repo{Name: "test", URI: "unused", Branch: "main"}
	_, err := Fetch(noread, r, true)
	if err == nil {
		t.Error("expected error for stat failure")
	}
}

func TestFetchExisting(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	// first call clones
	if _, err := Fetch(dest, r, true); err != nil {
		t.Fatal(err)
	}
	// second call fetches (should not error)
	if _, err := Fetch(dest, r, true); err != nil {
		t.Fatal(err)
	}
}

func TestFetchPicksUpNewCommits(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	path, err := Fetch(dest, r, true)
	if err != nil {
		t.Fatal(err)
	}

	// record HEAD before new commit
	old := gitRev(t, path, "main")

	// add a new commit to the source repo
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
	)
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "second")
	cmd.Dir = src
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %s: %s", err, out)
	}

	// fetch again
	if _, err := Fetch(dest, r, true); err != nil {
		t.Fatal(err)
	}

	// HEAD should have advanced
	cur := gitRev(t, path, "main")
	if old == cur {
		t.Error("fetch did not pick up new commit")
	}
}

func gitRev(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return string(out)
}
