package index

import (
	"context"
	"fmt"
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
	path, err := Fetch(dest, r, true, nil)
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
	path, err := Fetch(dest, r, false, nil)
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
	_, err := Fetch(noread, r, true, nil)
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
	if _, err := Fetch(dest, r, true, nil); err != nil {
		t.Fatal(err)
	}
	// second call fetches (should not error)
	if _, err := Fetch(dest, r, true, nil); err != nil {
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
	path, err := Fetch(dest, r, true, nil)
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
	if _, err := Fetch(dest, r, true, nil); err != nil {
		t.Fatal(err)
	}

	// fetched ref should have advanced
	cur := gitRev(t, path, "main")
	if old == cur {
		t.Error("fetch did not pick up new commit")
	}
}

func TestFetchContextCanceledBootstrapCleansTempBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)
	dest := filepath.Join(tmp, "repos")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := Repo{Name: "test", URI: src, Branch: "main"}
	_, err := FetchContext(ctx, dest, r, true, nil)
	if err == nil {
		t.Fatal("expected canceled fetch error")
	}
	if _, err := os.Stat(filepath.Join(dest, "test.git")); !os.IsNotExist(err) {
		t.Fatalf("expected no final bare repo path got %v", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp cleanup got %v", len(entries))
	}
}

func TestFetchContextCanceledDuringBootstrapFetchCleansTempBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)
	dest := filepath.Join(tmp, "repos")
	mark := filepath.Join(tmp, "fetch-started")

	installBlockingGitWrapper(t, tmp, ".test.git.tmp-*", "fetch", mark)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := FetchContext(ctx, dest, Repo{Name: "test", URI: src, Branch: "main"}, true, nil)
		done <- err
	}()

	waitForFile(t, mark)
	cancel()

	if err := <-done; err == nil {
		t.Fatal("expected canceled fetch error")
	}
	if _, err := os.Stat(filepath.Join(dest, "test.git")); !os.IsNotExist(err) {
		t.Fatalf("expected no final bare repo path got %v", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp cleanup got %v", len(entries))
	}
}

func TestFetchContextCanceledBootstrapCleansTempNonBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)
	dest := filepath.Join(tmp, "repos")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := Repo{Name: "test", URI: src, Branch: "main"}
	_, err := FetchContext(ctx, dest, r, false, nil)
	if err == nil {
		t.Fatal("expected canceled fetch error")
	}
	if _, err := os.Stat(filepath.Join(dest, "test")); !os.IsNotExist(err) {
		t.Fatalf("expected no final worktree repo path got %v", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp cleanup got %v", len(entries))
	}
}

func TestFetchContextCanceledDuringBootstrapCloneCleansTempNonBare(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)
	dest := filepath.Join(tmp, "repos")
	mark := filepath.Join(tmp, "clone-started")

	installBlockingGitWrapper(t, tmp, ".test.tmp-*", "clone", mark)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := FetchContext(ctx, dest, Repo{Name: "test", URI: src, Branch: "main"}, false, nil)
		done <- err
	}()

	waitForFile(t, mark)
	cancel()

	if err := <-done; err == nil {
		t.Fatal("expected canceled clone error")
	}
	if _, err := os.Stat(filepath.Join(dest, "test")); !os.IsNotExist(err) {
		t.Fatalf("expected no final worktree repo path got %v", err)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp cleanup got %v", len(entries))
	}
}

func TestFetchBareRejectsFilePathBeforeGitRuns(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	cwdRepo := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(cwdRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
	)
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = cwdRepo
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init cwd repo: %s: %s", err, out)
	}

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dest, "test.git")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(cwdRepo); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	if _, err := Fetch(dest, r, true, nil); err == nil {
		t.Fatal("expected file path error")
	}

	cmd = exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwdRepo
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected cwd repo to stay untouched got %s", out)
	}
}

func TestFetchNonBareRejectsFilePathBeforeGitRuns(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepo(t, tmp)

	cwdRepo := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(cwdRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
	)
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = cwdRepo
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init cwd repo: %s: %s", err, out)
	}

	dest := filepath.Join(tmp, "repos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dest, "test")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(cwdRepo); err != nil {
		t.Fatal(err)
	}

	r := Repo{Name: "test", URI: src, Branch: "main"}
	if _, err := Fetch(dest, r, false, nil); err == nil {
		t.Fatal("expected file path error")
	}

	cmd = exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwdRepo
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected cwd repo to stay untouched got %s", out)
	}
}

func installBlockingGitWrapper(t *testing.T, tmp, match, verb, mark string) {
	t.Helper()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(binDir, "git")
	script := fmt.Sprintf(`#!/bin/sh
case "$(basename "$PWD")" in
  %s)
    if [ "$1" = "%s" ]; then
      : > "$MIROIR_FETCH_MARK"
      trap 'exit 0' TERM INT
      while :; do sleep 1; done
    fi
    ;;
esac
exec "%s" "$@"
`, match, verb, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIROIR_FETCH_MARK", mark)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
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
