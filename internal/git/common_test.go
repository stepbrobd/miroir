package git

import (
	"testing"

	"ysun.co/miroir/internal/context"
)

func TestAvailable(t *testing.T) {
	if err := Available(); err != nil {
		t.Skipf("git not in PATH: %s", err)
	}
}

func TestRemoteIndex(t *testing.T) {
	ctx := &context.Context{
		Push: []context.Remote{
			{Name: "github", URI: "git@github.com:a/b"},
			{Name: "gitlab", URI: "git@gitlab.com:a/b"},
		},
	}
	if i := remoteIndex(ctx, "github"); i != 0 {
		t.Errorf("got %d, want 0", i)
	}
	if i := remoteIndex(ctx, "gitlab"); i != 1 {
		t.Errorf("got %d, want 1", i)
	}
	if i := remoteIndex(ctx, "missing"); i != -1 {
		t.Errorf("got %d, want -1", i)
	}
}

func TestRepoName(t *testing.T) {
	if got := repoName("/home/user/ws/myrepo"); got != "myrepo" {
		t.Errorf("got %q, want %q", got, "myrepo")
	}
}

func TestEnsureRepoMissing(t *testing.T) {
	err := ensureRepo(t.TempDir())
	if err == nil {
		t.Error("expected error for missing .git")
	}
}

func TestIsDirty(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	// init a clean repo
	if err := run(dir, nil, true, nil, "init"); err != nil {
		t.Fatal(err)
	}
	if isDirty(dir, nil) {
		t.Error("fresh repo should not be dirty")
	}
}
