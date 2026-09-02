package index

import (
	"context"
	"errors"
	"net"
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

// newCfg makes a daemon config with fresh repo and shard dirs under tmp
func newCfg(t *testing.T, tmp string) *Cfg {
	t.Helper()
	c := &Cfg{
		Listen:   ":0",
		Database: filepath.Join(tmp, "shards"),
		Interval: time.Hour,
		Bare:     true,
		Home:     filepath.Join(tmp, "repos"),
	}
	for _, dir := range []string{c.Home, c.Database} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

// seed is the managed repo every cycle test mirrors from src
func seed(src, branch string) Repo {
	return Repo{Name: "seed", IndexName: "seed", URI: src, Branch: branch}
}

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

// seedWithFeature adds a feature branch carrying feature.txt and returns to main
func seedWithFeature(t *testing.T, tmp string) string {
	t.Helper()
	env := gitEnv()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	gitRun(t, src, env, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(src, "feature.txt"), []byte("feature branch needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, src, env, "add", "feature.txt")
	gitRun(t, src, env, "commit", "-m", "add feature file")
	gitRun(t, src, env, "checkout", "main")
	return src
}

// includeRepo creates tmp/include/myrepo with one committed go file
func includeRepo(t *testing.T, tmp string) (string, string) {
	t.Helper()
	incDir := filepath.Join(tmp, "include")
	repoDir := filepath.Join(incDir, "myrepo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	env := gitEnv()
	gitRun(t, repoDir, env, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(repoDir, "lib.go"), []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repoDir, env, "add", ".")
	gitRun(t, repoDir, env, "commit", "-m", "init")
	return incDir, repoDir
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
	out, err := gitOutput(t.Context(), dir, env, "symbolic-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func refNames(t *testing.T, dir string, env []string, prefix string) []string {
	t.Helper()
	refs, err := listRefs(t.Context(), dir, env, prefix)
	if err != nil {
		t.Fatal(err)
	}
	for i, ref := range refs {
		refs[i] = strings.TrimPrefix(ref, prefix+"/")
	}
	return refs
}

func TestCycleIntegration(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("got shard repo names %v want [seed]", got)
	}
}

func TestCycleContextCanceledBeforeWork(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cycle(ctx, c)
	if _, err := os.Stat(filepath.Join(c.Home, "seed.git")); !os.IsNotExist(err) {
		t.Fatalf("expected canceled cycle to skip repo setup got %v", err)
	}
	if got := shardRepoNames(t, c.Database); len(got) != 0 {
		t.Fatalf("expected canceled cycle to skip shard writes got %v", got)
	}
}

func TestCycleContextCanceledDuringFetchStopsLaterRepos(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	first := seedRepoWithFile(t, tmp, "first.go", "package first\n")
	second := seedRepoWithFile(t, filepath.Join(tmp, "second-src"), "second.go", "package second\n")

	mark := filepath.Join(tmp, "fetch-started")
	installBlockingGitWrapper(t, tmp, ".first.git.tmp-*", "fetch", mark)

	c := newCfg(t, tmp)
	c.Env = gitEnv()
	c.Repos = []Repo{
		{Name: "first", IndexName: "first", URI: first, Branch: "main"},
		{Name: "second", IndexName: "second", URI: second, Branch: "main"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		cycle(ctx, c)
		close(done)
	}()

	waitForFile(t, mark)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for canceled cycle")
	}

	if _, err := os.Stat(filepath.Join(c.Home, "second.git")); !os.IsNotExist(err) {
		t.Fatalf("expected second repo not to start got %v", err)
	}
	if entries, err := os.ReadDir(c.Home); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("expected canceled cycle to clean temp repos got %v", len(entries))
	}
	if got := shardRepoNames(t, c.Database); len(got) != 0 {
		t.Fatalf("expected canceled cycle to skip shard writes got %v", got)
	}
}

func TestRunCancelWaitsForActiveIndex(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	seedRepoWithFile(t, tmp, "hello.go", "package main\n")
	c := newCfg(t, tmp)
	c.Include = []string{tmp}

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	oldIndexRepo := indexRepo
	indexRepo = func(repoDir, indexDir, name string) error {
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
	go func() { done <- Run(ctx, c) }()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for index start")
	}

	cancel()

	select {
	case err := <-done:
		t.Fatalf("expected shutdown to wait for active index got %v", err)
	case <-time.After(250 * time.Millisecond):
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for blocked index to finish")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("got %v want a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}

func TestRunReturnsServerErrorWithoutWaitingForFullCycle(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")

	// the bootstrap fetch blocks until killed, so Run can only return
	// promptly if the server failure cancels the in-flight cycle
	mark := filepath.Join(tmp, "fetch-started")
	installBlockingGitWrapper(t, tmp, ".seed.git.tmp-*", "fetch", mark)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	c := newCfg(t, tmp)
	c.Listen = ln.Addr().String()
	c.Env = gitEnv()
	c.Repos = []Repo{seed(src, "main")}

	done := make(chan error, 1)
	go func() { done <- Run(t.Context(), c) }()

	select {
	case err := <-done:
		if err == nil || errors.Is(err, context.Canceled) {
			t.Fatalf("expected bind error, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after server failure")
	}
	if _, err := os.Stat(filepath.Join(c.Home, "seed.git")); !os.IsNotExist(err) {
		t.Fatalf("expected aborted cycle to leave no repo, got %v", err)
	}
}

func TestCycleWithInclude(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	incDir, _ := includeRepo(t, tmp)
	c := newCfg(t, tmp)
	c.Include = []string{incDir}

	cycle(t.Context(), c)
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"myrepo"}) {
		t.Fatalf("got shard repo names %v want [myrepo]", got)
	}
}

func TestCycleCleansUpRemovedIncludeShards(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	incDir, repoDir := includeRepo(t, tmp)
	c := newCfg(t, tmp)
	c.Include = []string{incDir}

	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "package lib"); len(matches) == 0 {
		t.Fatal("expected indexed include content before cleanup")
	}

	if err := os.RemoveAll(repoDir); err != nil {
		t.Fatal(err)
	}
	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "package lib"); len(matches) != 0 {
		t.Fatalf("expected no include matches after cleanup got %v", matches)
	}
	if got := shardRepoNames(t, c.Database); len(got) != 0 {
		t.Fatalf("expected no include shards after cleanup got %v", got)
	}
}

func TestCycleKeepsIncludeShardsWhenDiscoveryFails(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	incDir, _ := includeRepo(t, tmp)
	c := newCfg(t, tmp)
	c.Include = []string{incDir}

	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "package lib"); len(matches) == 0 {
		t.Fatal("expected indexed include content")
	}

	// removing the include root makes discovery fail, shards must survive
	if err := os.RemoveAll(incDir); err != nil {
		t.Fatal(err)
	}
	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "package lib"); len(matches) == 0 {
		t.Fatal("expected include shards to survive discovery failure")
	}
}

func TestCycleRemovesOrphanedTempDirs(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "hello.go", "package main\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	orphan := filepath.Join(c.Home, ".seed.git.tmp-abc123")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(c.Home, ".other.tmp-x")
	if err := os.MkdirAll(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}

	cycle(t.Context(), c)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("expected orphaned temp dir removed got %v", err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatalf("expected unrelated hidden dir kept got %v", err)
	}
}

func TestCycleRemovesMarkedTempDirOfRemovedRepo(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	c := newCfg(t, tmp)

	// a crash orphan for a repo no longer in config carries the marker
	orphan := filepath.Join(c.Home, ".gone.git.tmp-xyz")
	gitRun(t, tmp, gitEnv(), "init", "--bare", orphan)
	gitRun(t, orphan, gitEnv(), "config", "miroir.managed", "true")

	cycle(t.Context(), c)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("expected marked temp dir removed got %v", err)
	}
}

func TestCycleBareReconcilesHeadsAndIndexesConfiguredBranch(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedWithFeature(t, tmp)
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "feature")}

	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "feature branch needle"); len(matches) == 0 {
		t.Fatal("expected feature branch content from configured bare head")
	}

	barePath := filepath.Join(c.Home, "seed.git")
	if got := bareHeadRef(t, barePath, env); got != "refs/heads/feature" {
		t.Fatalf("got HEAD %q want refs/heads/feature", got)
	}
	if got := refNames(t, barePath, env, "refs/heads"); !slices.Equal(got, []string{"feature", "main"}) {
		t.Fatalf("got local branches %v want [feature main]", got)
	}
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("got shard repo names %v want [seed]", got)
	}
}

func TestCycleBarePrunesDeletedOriginBranchesAndUnexpectedLocalHeads(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedWithFeature(t, tmp)
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	barePath := filepath.Join(c.Home, "seed.git")
	gitRun(t, barePath, env, "update-ref", "refs/heads/junk", "refs/heads/main")
	gitRun(t, src, env, "branch", "-D", "feature")

	cycle(t.Context(), c)
	if got := refNames(t, barePath, env, "refs/heads"); !slices.Equal(got, []string{"main"}) {
		t.Fatalf("got local branches %v want [main]", got)
	}
}

func TestCycleBareDropsTrackingRefsOfOlderDaemon(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	barePath := filepath.Join(c.Home, "seed.git")
	gitRun(t, barePath, env, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	gitRun(t, barePath, env, "update-ref", "refs/remotes/origin/main", "refs/heads/main")

	cycle(t.Context(), c)
	if got := refNames(t, barePath, env, "refs/remotes"); len(got) != 0 {
		t.Fatalf("got tracking refs %v want none", got)
	}
	if got := refNames(t, barePath, env, "refs/heads"); !slices.Equal(got, []string{"main"}) {
		t.Fatalf("got local branches %v want [main]", got)
	}
}

func TestCycleNonBareClonesConfiguredBranchThenFollowsHead(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	env := gitEnv()
	src := seedWithFeature(t, tmp)
	c := newCfg(t, tmp)
	c.Bare = false
	c.Repos = []Repo{seed(src, "feature")}

	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "feature branch needle"); len(matches) == 0 {
		t.Fatal("expected feature branch content from initial configured clone")
	}

	clone := filepath.Join(c.Home, "seed")
	gitRun(t, clone, env, "checkout", "-b", "main", "origin/main")

	cycle(t.Context(), c)
	if matches := searchMatches(t, c.Database, "feature branch needle"); len(matches) != 0 {
		t.Fatalf("got stale feature matches after switching local HEAD: %v", matches)
	}
}

func TestCycleCleansUpRemovedManagedRepoAndShards(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	if _, err := os.Stat(filepath.Join(c.Home, "seed.git")); err != nil {
		t.Fatal(err)
	}
	if matches := searchMatches(t, c.Database, "main branch only"); len(matches) == 0 {
		t.Fatal("expected indexed content before cleanup")
	}

	c.Repos = nil
	cycle(t.Context(), c)
	if _, err := os.Stat(filepath.Join(c.Home, "seed.git")); !os.IsNotExist(err) {
		t.Fatalf("expected managed repo dir removed got %v", err)
	}
	if matches := searchMatches(t, c.Database, "main branch only"); len(matches) != 0 {
		t.Fatalf("expected no matches after cleanup got %v", matches)
	}
	if got := shardRepoNames(t, c.Database); len(got) != 0 {
		t.Fatalf("expected no managed shards after cleanup got %v", got)
	}
}

func TestCycleCleanupKeepsUnmanagedRepoDirs(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	c := newCfg(t, tmp)

	unmanaged := filepath.Join(c.Home, "unmanaged.git")
	gitRun(t, tmp, gitEnv(), "init", "--bare", unmanaged)

	cycle(t.Context(), c)
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("expected unmanaged repo to remain got %v", err)
	}
}

func TestCycleRemovesLegacyManagedShardNames(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	barePath := filepath.Join(c.Home, "seed.git")
	gitRun(t, barePath, gitEnv(), "config", "zoekt.name", "seed.git")
	if err := IndexRepo(barePath, c.Database, "seed.git"); err != nil {
		t.Fatal(err)
	}
	gitRun(t, barePath, gitEnv(), "config", "zoekt.name", "seed")
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed", "seed.git"}) {
		t.Fatalf("expected legacy shard alongside managed shard got %v", got)
	}

	cycle(t.Context(), c)
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("expected legacy shard removed got %v", got)
	}
}

func TestCleanupManagedShardsForRepoRemovesLegacyNames(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{seed(src, "main")}

	cycle(t.Context(), c)
	repoPath := filepath.Join(c.Home, "seed.git")
	gitRun(t, repoPath, gitEnv(), "config", "zoekt.name", "seed.git")
	if err := IndexRepo(repoPath, c.Database, "seed.git"); err != nil {
		t.Fatal(err)
	}
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed", "seed.git"}) {
		t.Fatalf("expected duplicate shard names got %v", got)
	}

	if err := cleanupManagedShardsForRepo(c.Database, repoPath, "seed"); err != nil {
		t.Fatal(err)
	}
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, []string{"seed"}) {
		t.Fatalf("expected legacy shard removed got %v", got)
	}
}

func TestCycleRemovesNamespacedShardsWithMovedSource(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Namespace = "github.com/alice/"
	c.Repos = []Repo{{Name: "seed", IndexName: "github.com/alice/seed", URI: src, Branch: "main"}}

	// both shards point at a source outside home, only the namespaced
	// name of a repo no longer in config is daemon owned
	if err := IndexRepo(src, c.Database, "github.com/alice/gone"); err != nil {
		t.Fatal(err)
	}
	if err := IndexRepo(src, c.Database, "other.org/bob/kept"); err != nil {
		t.Fatal(err)
	}

	cycle(t.Context(), c)
	want := []string{"github.com/alice/seed", "other.org/bob/kept"}
	if got := shardRepoNames(t, c.Database); !slices.Equal(got, want) {
		t.Fatalf("got shard repo names %v want %v", got, want)
	}
}

func TestCycleManagedRepoUsesFullNameAndGithubLinks(t *testing.T) {
	skipNoGit(t)
	tmp := t.TempDir()
	src := seedRepoWithFile(t, tmp, "main.txt", "main branch only\n")
	c := newCfg(t, tmp)
	c.Repos = []Repo{{
		Name:       "seed",
		IndexName:  "github.com/alice/seed",
		URI:        src,
		Branch:     "main",
		WebURL:     "https://github.com/alice/seed",
		WebURLType: "github",
	}}

	cycle(t.Context(), c)

	repo := shardRepoByName(t, c.Database, "github.com/alice/seed")
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
	if got.Namespace != "github.com/alice/" {
		t.Errorf("namespace: got %q", got.Namespace)
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
	if !slices.Contains(got.Env, "FROM_SHELL=shell") {
		t.Errorf("expected shell env precedence, got %v", got.Env)
	}
	if !slices.Contains(got.Env, "ONLY_CONFIG=yes") {
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
