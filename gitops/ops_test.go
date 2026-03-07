package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestExecNoArgs(t *testing.T) {
	op := Exec{}
	err := op.Run(Params{Path: t.TempDir(), Ctx: &workspace.Context{Env: os.Environ()}})
	if err == nil {
		t.Fatal("expected error when no command is provided")
	}
}

func TestPullDirtyWithoutForce(t *testing.T) {
	v := false
	disp := display.New(1, 1, display.DefaultTheme, &v)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Pull{}.Run(Params{Path: dir, Ctx: &workspace.Context{Env: gitEnv(), Branch: "main"}, Disp: disp, Sem: make(chan struct{}, 1)})
	if err == nil {
		t.Fatal("expected dirty tree error")
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
	if err := run(remote, env, true, nil, "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := run(remote, env, true, nil, "commit", "--allow-empty", "-m", "init"); err != nil {
		t.Fatal(err)
	}
	if err := run(tmp, env, true, nil, "clone", remote, local); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(remote, "conflict.txt"), []byte("remote\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(remote, env, true, nil, "add", "conflict.txt"); err != nil {
		t.Fatal(err)
	}
	if err := run(remote, env, true, nil, "commit", "-m", "add conflict"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(local, "conflict.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := false
	disp := display.New(1, 1, display.DefaultTheme, &v)
	err := Pull{}.Run(Params{
		Path: local,
		Ctx:  &workspace.Context{Env: env, Branch: "main"},
		Disp: disp,
		Sem:  make(chan struct{}, 1),
	})
	if err == nil {
		t.Fatal("expected dirty tree error")
	}

	err = Pull{}.Run(Params{
		Path:  local,
		Ctx:   &workspace.Context{Env: env, Branch: "main"},
		Disp:  disp,
		Sem:   make(chan struct{}, 1),
		Force: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("git", "-C", local, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	want, err := exec.Command("git", "-C", remote, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(want) {
		t.Fatalf("local head = %s want %s", out, want)
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
	if err := run(sub, env, true, nil, "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.txt"), []byte("sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(sub, env, true, nil, "add", "file.txt"); err != nil {
		t.Fatal(err)
	}
	if err := run(sub, env, true, nil, "commit", "-m", "init submodule"); err != nil {
		t.Fatal(err)
	}

	parent := filepath.Join(tmp, "parent")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := run(parent, env, true, nil, "init", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := run(parent, env, true, nil,
		"-c", "protocol.file.allow=always", "submodule", "add", sub, "deps/sub"); err != nil {
		t.Fatal(err)
	}
	if err := run(parent, env, true, nil, "add", "."); err != nil {
		t.Fatal(err)
	}
	if err := run(parent, env, true, nil, "commit", "-m", "add submodule"); err != nil {
		t.Fatal(err)
	}

	local := filepath.Join(tmp, "local")
	v := false
	disp := display.New(1, 1, display.DefaultTheme, &v)
	err := Init{}.Run(Params{
		Path: local,
		Ctx: &workspace.Context{
			Env:    env,
			Branch: "main",
			Fetch:  []workspace.Remote{{Name: "origin", GitName: "origin", URI: parent}},
			Push:   []workspace.Remote{{Name: "origin", GitName: "origin", URI: parent}},
		},
		Disp: disp,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(local, "deps", "sub", "file.txt")); err != nil {
		t.Fatalf("expected populated submodule: %v", err)
	}
}
