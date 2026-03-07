package index

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	zoekt "github.com/sourcegraph/zoekt"
	zoektindex "github.com/sourcegraph/zoekt/index"
	"github.com/sourcegraph/zoekt/query"
	"github.com/sourcegraph/zoekt/search"

	"ysun.co/miroir/config"
)

// seedRepoWithFile creates a git repo with one committed file
func seedRepoWithFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	src := filepath.Join(dir, "seed")
	os.MkdirAll(src, 0o755)
	env := gitEnv()
	run := func(args ...string) { gitRun(t, src, env, args...) }
	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(src, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "add file")
	return src
}

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_ALLOW_PROTOCOL=file",
	)
}

func gitRun(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %s", args, err, out)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func searchMatches(t *testing.T, dir, pattern string) []zoekt.FileMatch {
	t.Helper()
	searcher, err := search.NewDirectorySearcher(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer searcher.Close()

	result, err := searcher.Search(context.Background(),
		&query.Substring{Pattern: pattern},
		&zoekt.SearchOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return result.Files
}

func shardRepoNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".zoekt" {
			continue
		}
		repos, _, err := zoektindex.ReadMetadataPath(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, repo := range repos {
			set[repo.Name] = struct{}{}
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func shardRepoByName(t *testing.T, dir, name string) *zoekt.Repository {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".zoekt" {
			continue
		}
		repos, _, err := zoektindex.ReadMetadataPath(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, repo := range repos {
			if repo.Name == name {
				return repo
			}
		}
	}
	t.Fatalf("repo %q not found in shard metadata", name)
	return nil
}

func bareHeadRef(t *testing.T, dir string, env []string) string {
	t.Helper()
	out, err := gitOutput(dir, CmdEnv(env), "symbolic-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func branchNames(t *testing.T, dir string, env []string, prefix string, strip int) []string {
	t.Helper()
	names, err := listRefs(dir, CmdEnv(env), prefix, strip)
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestCycleIntegration(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)

	// verify shards were created
	entries, err := os.ReadDir(db)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".zoekt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no .zoekt shard files created")
	}
}

func TestCycleContextCanceledBeforeWork(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycleContext(ctx, c)
	if _, err := os.Stat(filepath.Join(home, "seed.git")); !os.IsNotExist(err) {
		t.Fatalf("expected canceled cycle to skip repo setup got %v", err)
	}
	if got := shardRepoNames(t, db); len(got) != 0 {
		t.Fatalf("expected canceled cycle to skip shard writes got %v", got)
	}
}

func TestCycleContextCanceledDuringFetchStopsLaterRepos(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()

	first := seedRepoWithFile(t, tmp, "first.go", "package first\n")
	second := seedRepoWithFile(t, filepath.Join(tmp, "second-src"), "second.go", "package second\n")

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(tmp, "fetch-started")
	wrapper := filepath.Join(binDir, "git")
	script := fmt.Sprintf(`#!/bin/sh
case "$(basename "$PWD")" in
  .first.git.tmp-*)
    if [ "$1" = "fetch" ]; then
  : > "$MIROIR_FETCH_MARK"
  trap 'exit 0' TERM INT
  while :; do sleep 1; done
    fi
    ;;
esac
exec "%s" "$@"
`, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MIROIR_FETCH_MARK", mark)
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Env:      CmdEnv(gitEnv()),
		Repos: []Repo{
			{Name: "first", URI: first, Branch: "main"},
			{Name: "second", URI: second, Branch: "main"},
		},
	}

	done := make(chan struct{})
	go func() {
		cycleContext(ctx, c)
		close(done)
	}()

	waitForFile(t, mark)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for canceled cycle")
	}

	if _, err := os.Stat(filepath.Join(home, "second.git")); !os.IsNotExist(err) {
		t.Fatalf("expected second repo not to start got %v", err)
	}
	if got := shardRepoNames(t, db); len(got) != 0 {
		t.Fatalf("expected canceled cycle to skip shard writes got %v", got)
	}
}

func TestRunCancelDoesNotWaitForActiveIndex(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	seedRepoWithFile(t, tmp, "hello.go", "package main\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	oldIndexRepo := indexRepo
	indexRepo = func(repoDir, indexDir, name string, branches []string) error {
		close(started)
		<-release
		close(finished)
		return nil
	}
	t.Cleanup(func() {
		indexRepo = oldIndexRepo
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, &Cfg{
			Listen:   ":0",
			Database: db,
			Interval: time.Hour,
			Home:     home,
			Include:  []string{tmp},
		})
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for index start")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for blocked index to finish")
	}
}

func TestCycleWithInclude(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()

	// create a repo inside an include dir
	incDir := filepath.Join(tmp, "include")
	repoDir := filepath.Join(incDir, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := gitEnv()
	run := func(args ...string) { gitRun(t, repoDir, env, args...) }
	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(repoDir, "lib.go"), []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     filepath.Join(tmp, "managed"),
		Include:  []string{incDir},
	}

	cycle(c)

	entries, err := os.ReadDir(db)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".zoekt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("no .zoekt shard files created from include path")
	}
}

func TestCycleCleansUpRemovedIncludeShards(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()

	incDir := filepath.Join(tmp, "include")
	repoDir := filepath.Join(incDir, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	env := gitEnv()
	run := func(args ...string) { gitRun(t, repoDir, env, args...) }
	run("init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(repoDir, "lib.go"), []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     filepath.Join(tmp, "managed"),
		Include:  []string{incDir},
	}

	cycle(c)
	if matches := searchMatches(t, db, "package lib"); len(matches) == 0 {
		t.Fatal("expected indexed include content before cleanup")
	}

	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatal(err)
	}
	cycle(c)
	if matches := searchMatches(t, db, "package lib"); len(matches) != 0 {
		t.Fatalf("expected no include matches after cleanup got %v", matches)
	}
	if got := shardRepoNames(t, db); len(got) != 0 {
		t.Fatalf("expected no include shards after cleanup got %v", got)
	}
}

func TestCycleBareReconcilesHeadsAndIndexesConfiguredBranch(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	gitRun(t, src, env, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(src, "feature.txt"), []byte("feature branch needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, env, "add", "feature.txt")
	gitRun(t, src, env, "commit", "-m", "add feature file")
	gitRun(t, src, env, "checkout", "main")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "feature"}},
	}

	cycle(c)
	if matches := searchMatches(t, db, "feature branch needle"); len(matches) == 0 {
		t.Fatal("expected feature branch content from configured bare head")
	}

	barePath := filepath.Join(home, "seed.git")
	if got := bareHeadRef(t, barePath, env); got != "refs/heads/feature" {
		t.Fatalf("got HEAD %q want refs/heads/feature", got)
	}
	if got := branchNames(t, barePath, env, "refs/heads", 2); !slices.Equal(got, []string{"feature", "main"}) {
		t.Fatalf("got local branches %v want [feature main]", got)
	}
	if got := shardRepoNames(t, db); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("got shard repo names %v want [seed]", got)
	}
}

func TestCycleBarePrunesDeletedOriginBranchesAndUnexpectedLocalHeads(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	gitRun(t, src, env, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(src, "feature.txt"), []byte("feature branch needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, env, "add", "feature.txt")
	gitRun(t, src, env, "commit", "-m", "add feature file")
	gitRun(t, src, env, "checkout", "main")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)
	barePath := filepath.Join(home, "seed.git")
	hash, err := resolveRef(barePath, CmdEnv(env), "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, barePath, env, "update-ref", "refs/heads/junk", hash)
	gitRun(t, src, env, "branch", "-D", "feature")

	cycle(c)
	if got := branchNames(t, barePath, env, "refs/heads", 2); !slices.Equal(got, []string{"main"}) {
		t.Fatalf("got local branches %v want [main]", got)
	}
}

func TestCycleNonBareClonesConfiguredBranchThenFollowsHead(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	gitRun(t, src, env, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(src, "feature.txt"), []byte("feature branch needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, env, "add", "feature.txt")
	gitRun(t, src, env, "commit", "-m", "add feature file")
	gitRun(t, src, env, "checkout", "main")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     false,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "feature"}},
	}

	cycle(c)
	if matches := searchMatches(t, db, "feature branch needle"); len(matches) == 0 {
		t.Fatal("expected feature branch content from initial configured clone")
	}

	clone := filepath.Join(home, "seed")
	gitRun(t, clone, env, "checkout", "-b", "main", "origin/main")

	cycle(c)
	if matches := searchMatches(t, db, "feature branch needle"); len(matches) != 0 {
		t.Fatalf("got stale feature matches after switching local HEAD: %v", matches)
	}
}

func TestCycleCleansUpRemovedManagedRepoAndShards(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)
	if _, err := os.Stat(filepath.Join(home, "seed.git")); err != nil {
		t.Fatal(err)
	}
	if matches := searchMatches(t, db, "main branch only"); len(matches) == 0 {
		t.Fatal("expected indexed content before cleanup")
	}

	c.Repos = nil
	cycle(c)
	if _, err := os.Stat(filepath.Join(home, "seed.git")); !os.IsNotExist(err) {
		t.Fatalf("expected managed repo dir removed got %v", err)
	}
	if matches := searchMatches(t, db, "main branch only"); len(matches) != 0 {
		t.Fatalf("expected no matches after cleanup got %v", matches)
	}
	if got := shardRepoNames(t, db); len(got) != 0 {
		t.Fatalf("expected no managed shards after cleanup got %v", got)
	}
}

func TestCycleCleanupKeepsUnmanagedRepoDirs(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	unmanaged := filepath.Join(home, "unmanaged.git")
	gitRun(t, tmp, env, "init", "--bare", unmanaged)

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
	}

	cycle(c)
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("expected unmanaged repo to remain got %v", err)
	}
}

func TestCycleRemovesLegacyManagedShardNames(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)
	barePath := filepath.Join(home, "seed.git")
	gitRun(t, barePath, gitEnv(), "config", "zoekt.name", "seed.git")
	if err := IndexRepo(barePath, db, "seed.git", nil); err != nil {
		t.Fatal(err)
	}
	gitRun(t, barePath, gitEnv(), "config", "zoekt.name", "seed")
	if got := shardRepoNames(t, db); !slices.Equal(got, []string{"seed", "seed.git"}) {
		t.Fatalf("expected legacy shard alongside managed shard got %v", got)
	}

	cycle(c)
	if got := shardRepoNames(t, db); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("expected legacy shard removed got %v", got)
	}
}

func TestCleanupManagedShardsForRepoRemovesLegacyNames(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)
	repoPath := filepath.Join(home, "seed.git")
	gitRun(t, repoPath, gitEnv(), "config", "zoekt.name", "seed.git")
	if err := IndexRepo(repoPath, db, "seed.git", nil); err != nil {
		t.Fatal(err)
	}
	if got := shardRepoNames(t, db); !slices.Equal(got, []string{"seed", "seed.git"}) {
		t.Fatalf("expected duplicate shard names got %v", got)
	}

	if err := cleanupManagedShardsForRepo(db, repoPath, "seed"); err != nil {
		t.Fatal(err)
	}
	if got := shardRepoNames(t, db); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("expected legacy shard removed got %v", got)
	}
}

func TestCycleManagedRepoUsesFullNameAndGithubLinks(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")

	home := filepath.Join(tmp, "repos")
	db := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(db, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Cfg{
		Listen:   ":0",
		Database: db,
		Interval: time.Hour,
		Bare:     true,
		Home:     home,
		Repos: []Repo{{
			Name:       "seed",
			IndexName:  "github.com/alice/seed",
			URI:        src,
			Branch:     "main",
			WebURL:     "https://github.com/alice/seed",
			WebURLType: "github",
		}},
	}

	cycle(c)

	repo := shardRepoByName(t, db, "github.com/alice/seed")
	if repo.Name != "github.com/alice/seed" {
		t.Fatalf("name: got %q", repo.Name)
	}
	if repo.URL != "https://github.com/alice/seed" {
		t.Fatalf("url: got %q", repo.URL)
	}
	if !strings.Contains(repo.CommitURLTemplate, "https://github.com/alice/seed") {
		t.Fatalf("commit template: got %q", repo.CommitURLTemplate)
	}
	if !strings.Contains(repo.FileURLTemplate, "https://github.com/alice/seed") {
		t.Fatalf("file template: got %q", repo.FileURLTemplate)
	}
}

func TestCfgFromValidation(t *testing.T) {
	t.Setenv("HOME", "/tmp/test")

	// interval = 0 should fail
	c := &config.Config{
		General: config.General{Home: "/tmp", Branch: "main"},
		Index:   config.Index{Listen: ":0", Database: "/tmp/db", Interval: 0, Bare: true},
	}
	_, err := CfgFrom(c)
	if err == nil {
		t.Error("expected error for interval=0")
	}

	// negative interval should fail
	c.Index.Interval = -1
	_, err = CfgFrom(c)
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestCfgFromBasic(t *testing.T) {
	t.Setenv("HOME", "/tmp/test")
	t.Setenv("FROM_SHELL", "shell")

	c := &config.Config{
		General: config.General{Home: "/tmp/ws", Branch: "main", Env: map[string]string{"FROM_SHELL": "config", "ONLY_CONFIG": "yes"}},
		Platform: map[string]config.Platform{
			"gh": {Origin: true, Domain: "github.com", User: "alice"},
		},
		Repo: map[string]config.Repo{
			"foo": {Visibility: config.Public},
		},
		Index: config.Index{
			Listen: ":8080", Database: "/tmp/db",
			Interval: 60, Bare: true,
		},
	}

	got, err := CfgFrom(c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Listen != ":8080" {
		t.Errorf("listen: got %q", got.Listen)
	}
	if got.Database != "/tmp/db" {
		t.Errorf("database: got %q", got.Database)
	}
	if got.Interval != 60*time.Second {
		t.Errorf("interval: got %v", got.Interval)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos: got %d, want 1", len(got.Repos))
	}
	if got.Repos[0].Name != "foo" {
		t.Errorf("repo name: got %q", got.Repos[0].Name)
	}
	if got.Repos[0].IndexName != "github.com/alice/foo" {
		t.Errorf("repo index name: got %q", got.Repos[0].IndexName)
	}
	if got.Repos[0].WebURL != "https://github.com/alice/foo" {
		t.Errorf("repo web url: got %q", got.Repos[0].WebURL)
	}
	if got.Repos[0].WebURLType != "github" {
		t.Errorf("repo web url type: got %q", got.Repos[0].WebURLType)
	}
	if !slices.Contains([]string(got.Env), "FROM_SHELL=shell") {
		t.Errorf("expected shell env precedence, got %v", got.Env)
	}
	if !slices.Contains([]string(got.Env), "ONLY_CONFIG=yes") {
		t.Errorf("expected config env to be merged, got %v", got.Env)
	}
}

func TestCfgFromSkipsArchived(t *testing.T) {
	t.Setenv("HOME", "/tmp/test")

	c := &config.Config{
		General: config.General{Home: "/tmp/ws", Branch: "main"},
		Platform: map[string]config.Platform{
			"gh": {Origin: true, Domain: "github.com", User: "alice"},
		},
		Repo: map[string]config.Repo{
			"active":   {Visibility: config.Public},
			"archived": {Visibility: config.Public, Archived: true},
		},
		Index: config.Index{
			Listen: ":0", Database: "/tmp/db",
			Interval: 60, Bare: true,
		},
	}

	got, err := CfgFrom(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos: got %d, want 1 (archived should be excluded)", len(got.Repos))
	}
	if got.Repos[0].Name != "active" {
		t.Errorf("repo name: got %q, want active", got.Repos[0].Name)
	}
}
