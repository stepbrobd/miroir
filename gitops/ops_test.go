package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ysun.co/miroir/display"
	"ysun.co/miroir/workspace"
)

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=t@t",
	)
}

func git(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	if err := runQuiet(t.Context(), dir, env, args...); err != nil {
		t.Fatal(err)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func plainDisplay(remotes int) *display.Display {
	v := false
	return display.New(1, remotes, display.DefaultTheme, &v)
}

func TestExecNoArgs(t *testing.T) {
	op := Exec{}
	err := op.Run(Params{RunCtx: t.Context(), Path: t.TempDir(), Ctx: &workspace.Context{Env: os.Environ()}})
	if err == nil {
		t.Fatal("expected error when no command is provided")
	}
}

func TestPullDirtyWithoutForce(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	env := gitEnv()
	git(t, dir, env, "init", "--initial-branch=main")
	git(t, dir, env, "commit", "--allow-empty", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Pull{}.Run(Params{
		RunCtx: t.Context(),
		Path:   dir,
		Ctx:    &workspace.Context{Env: env, Branch: "main"},
		Disp:   plainDisplay(1),
		Sem:    make(chan struct{}, 1),
	})
	if err == nil {
		t.Fatal("expected dirty tree error")
	}
	if !strings.Contains(err.Error(), "dirty working tree") {
		t.Fatalf("expected dirty tree error, got %v", err)
	}
}

func TestPullForceRemovesUntrackedConflict(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	remote := filepath.Join(tmp, "remote")
	local := filepath.Join(tmp, "local")
	env := gitEnv()

	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, remote, env, "init", "--initial-branch=main")
	git(t, remote, env, "commit", "--allow-empty", "-m", "init")
	git(t, tmp, env, "clone", remote, local)

	if err := os.WriteFile(filepath.Join(remote, "conflict.txt"), []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, remote, env, "add", "conflict.txt")
	git(t, remote, env, "commit", "-m", "add conflict")

	if err := os.WriteFile(filepath.Join(local, "conflict.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	disp := plainDisplay(1)
	err := Pull{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx:    &workspace.Context{Env: env, Branch: "main"},
		Disp:   disp,
		Sem:    make(chan struct{}, 1),
	})
	if err == nil {
		t.Fatal("expected dirty tree error")
	}

	err = Pull{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx:    &workspace.Context{Env: env, Branch: "main"},
		Disp:   disp,
		Sem:    make(chan struct{}, 1),
		Force:  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := gitOut(t, local, "rev-parse", "HEAD"), gitOut(t, remote, "rev-parse", "HEAD"); got != want {
		t.Fatalf("local head = %s want %s", got, want)
	}
}

func TestInitPopulatesSubmodules(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	env := append(gitEnv(), "GIT_ALLOW_PROTOCOL=file")

	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, sub, env, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, sub, env, "add", "file.txt")
	git(t, sub, env, "commit", "-m", "init submodule")

	parent := filepath.Join(tmp, "parent")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, parent, env, "init", "--initial-branch=main")
	git(t, parent, env,
		"-c", "protocol.file.allow=always", "submodule", "add", sub, "deps/sub")
	git(t, parent, env, "add", ".")
	git(t, parent, env, "commit", "-m", "add submodule")

	local := filepath.Join(tmp, "local")
	err := Init{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Origin: workspace.Remote{Name: "origin", GitName: "origin", URI: parent},
			Push:   []workspace.Remote{{Name: "origin", GitName: "origin", URI: parent}},
		},
		Disp: plainDisplay(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(local, "deps", "sub", "file.txt")); err != nil {
		t.Fatalf("expected populated submodule: %v", err)
	}
}

func TestInitDirtyExistingRepoRequiresForce(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	env := gitEnv()
	git(t, dir, env, "init", "--initial-branch=main")
	git(t, dir, env, "commit", "--allow-empty", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Init{}.Run(Params{
		RunCtx: t.Context(),
		Path:   dir,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
		},
		Disp: plainDisplay(1),
	})
	if err == nil {
		t.Fatal("expected dirty tree error")
	}
}

func TestInitForceResetsTrackedAndUntrackedChanges(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	env := gitEnv()
	remote := filepath.Join(tmp, "remote")
	local := filepath.Join(tmp, "local")

	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, remote, env, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(remote, "tracked.txt"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, remote, env, "add", "tracked.txt")
	git(t, remote, env, "commit", "-m", "init")
	git(t, tmp, env, "clone", remote, local)

	if err := os.WriteFile(filepath.Join(remote, "tracked.txt"), []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, remote, env, "add", "tracked.txt")
	git(t, remote, env, "commit", "-m", "remote update")

	if err := os.WriteFile(filepath.Join(local, "tracked.txt"), []byte("local dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "untracked.txt"), []byte("junk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Init{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Force:  true,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Origin: workspace.Remote{Name: "origin", GitName: "origin", URI: remote},
			Push:   []workspace.Remote{{Name: "origin", GitName: "origin", URI: remote}},
		},
		Disp: plainDisplay(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := os.ReadFile(filepath.Join(local, "tracked.txt")); err != nil {
		t.Fatal(err)
	} else if string(got) != "remote\n" {
		t.Fatalf("tracked.txt = %q want %q", got, "remote\n")
	}
	if _, err := os.Stat(filepath.Join(local, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected untracked file removed got %v", err)
	}

	if status := gitOut(t, local, "status", "--porcelain"); status != "" {
		t.Fatalf("expected clean worktree got %s", status)
	}

	if got, want := gitOut(t, local, "rev-parse", "HEAD"), gitOut(t, remote, "rev-parse", "HEAD"); got != want {
		t.Fatalf("local head = %s want %s", got, want)
	}
}

func TestFetchAllRemotes(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	env := gitEnv()
	r1 := filepath.Join(tmp, "r1")
	r2 := filepath.Join(tmp, "r2")
	local := filepath.Join(tmp, "local")

	for _, r := range []string{r1, r2} {
		if err := os.MkdirAll(r, 0o755); err != nil {
			t.Fatal(err)
		}
		git(t, r, env, "init", "--initial-branch=main")
		git(t, r, env, "commit", "--allow-empty", "-m", "init")
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, local, env, "init", "--initial-branch=main")
	git(t, local, env, "remote", "add", "origin", r1)
	git(t, local, env, "remote", "add", "gitlab", r2)

	err := Fetch{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Origin: workspace.Remote{Name: "github", GitName: "origin", URI: r1},
			Push: []workspace.Remote{
				{Name: "github", GitName: "origin", URI: r1},
				{Name: "gitlab", GitName: "gitlab", URI: r2},
			},
		},
		Disp: plainDisplay(2),
		Sem:  make(chan struct{}, 2),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []string{"refs/remotes/origin/main", "refs/remotes/gitlab/main"} {
		if gitOut(t, local, "rev-parse", "--verify", ref) == "" {
			t.Fatalf("expected %s to exist", ref)
		}
	}
}

func TestFetchReportsFailedRemote(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	env := gitEnv()
	local := filepath.Join(tmp, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, local, env, "init", "--initial-branch=main")
	git(t, local, env, "remote", "add", "origin", filepath.Join(tmp, "missing"))

	err := Fetch{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Push:   []workspace.Remote{{Name: "github", GitName: "origin", URI: filepath.Join(tmp, "missing")}},
		},
		Disp: plainDisplay(1),
		Sem:  make(chan struct{}, 1),
	})
	if err == nil {
		t.Fatal("expected fetch failure")
	}
	if !strings.Contains(err.Error(), "fetch from github failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushAllRemotes(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	env := gitEnv()
	r1 := filepath.Join(tmp, "r1.git")
	r2 := filepath.Join(tmp, "r2.git")
	local := filepath.Join(tmp, "local")

	for _, r := range []string{r1, r2} {
		git(t, tmp, env, "init", "--bare", "--initial-branch=main", r)
	}
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, local, env, "init", "--initial-branch=main")
	git(t, local, env, "commit", "--allow-empty", "-m", "init")
	git(t, local, env, "remote", "add", "origin", r1)
	git(t, local, env, "remote", "add", "gitlab", r2)

	err := Push{}.Run(Params{
		RunCtx: t.Context(),
		Path:   local,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Origin: workspace.Remote{Name: "github", GitName: "origin", URI: r1},
			Push: []workspace.Remote{
				{Name: "github", GitName: "origin", URI: r1},
				{Name: "gitlab", GitName: "gitlab", URI: r2},
			},
		},
		Disp: plainDisplay(2),
		Sem:  make(chan struct{}, 2),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := gitOut(t, local, "rev-parse", "main")
	for _, r := range []string{r1, r2} {
		if got := gitOut(t, r, "rev-parse", "main"); got != want {
			t.Fatalf("remote %s head = %s want %s", r, got, want)
		}
	}
}
