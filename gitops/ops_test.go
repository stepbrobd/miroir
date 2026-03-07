package git

import (
	"os"
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
	if err != nil {
		t.Fatal(err)
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
