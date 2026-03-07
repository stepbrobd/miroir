package index

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	zoekt "github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
	"github.com/sourcegraph/zoekt/search"

	"ysun.co/miroir/config"
)

// seedRepoWithFile creates a git repo with one committed file.
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

func TestCycleNonBareIndexesCheckedOutHead(t *testing.T) {
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
		Repos:    []Repo{{Name: "seed", URI: src, Branch: "main"}},
	}

	cycle(c)
	if matches := searchMatches(t, db, "feature branch needle"); len(matches) != 0 {
		t.Fatalf("got matches on initial main checkout: %v", matches)
	}

	clone := filepath.Join(home, "seed")
	gitRun(t, clone, env, "checkout", "-b", "feature", "origin/feature")

	cycle(c)
	matches := searchMatches(t, db, "feature branch needle")
	if len(matches) == 0 {
		t.Fatal("expected feature branch content after checking out feature locally")
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
