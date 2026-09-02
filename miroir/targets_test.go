package miroir

import (
	"os"
	"path/filepath"
	"testing"

	"ysun.co/miroir/config"
	"ysun.co/miroir/workspace"
)

func TestSelectTargetsByName(t *testing.T) {
	cfg := &config.Config{General: config.General{Home: "/tmp/ws"}}
	ctxs := []*workspace.Context{
		{Name: "alpha", Path: "/tmp/ws/alpha"},
		{Name: "beta", Path: "/tmp/ws/beta"},
	}
	got, err := SelectTargets(cfg, ctxs, SelectOptions{Name: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "/tmp/ws/beta" {
		t.Fatalf("got %v, want [/tmp/ws/beta]", got)
	}
}

func TestSelectTargetsAllSorted(t *testing.T) {
	cfg := &config.Config{General: config.General{Home: "/tmp/ws"}}
	ctxs := []*workspace.Context{
		{Name: "beta", Path: "/tmp/ws/beta"},
		{Name: "alpha", Path: "/tmp/ws/alpha"},
	}
	got, err := SelectTargets(cfg, ctxs, SelectOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Fatalf("got %v, want sorted [alpha beta]", got)
	}
}

func TestResolveNamesUnknownName(t *testing.T) {
	if _, err := resolveNames([]string{"alpha"}, "/tmp/ws", SelectOptions{Name: "beta"}); err == nil {
		t.Fatal("expected unknown repo error")
	}
}

func TestResolveNamesUnmanagedCwd(t *testing.T) {
	if _, err := resolveNames([]string{"alpha"}, "/tmp/ws", SelectOptions{Cwd: t.TempDir()}); err == nil {
		t.Fatal("expected unmanaged cwd error")
	}
}

func TestResolveNamesMatchesSymlinkedWorkspaceCwd(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	link := filepath.Join(tmp, "link")
	if err := os.MkdirAll(filepath.Join(real, "ws", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	got, err := resolveNames([]string{"alpha"}, filepath.Join(link, "ws"), SelectOptions{
		Cwd: filepath.Join(real, "ws", "alpha"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("got %v want [alpha]", got)
	}
}

func TestSyncNamesIncludesArchived(t *testing.T) {
	cfg := &config.Config{
		General: config.General{Home: "/tmp/ws"},
		Repo: map[string]config.Repo{
			"live": {},
			"old":  {Archived: true},
		},
	}
	got, err := SyncNames(cfg, SelectOptions{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "live" || got[1] != "old" {
		t.Fatalf("got %v, want [live old]", got)
	}
}
