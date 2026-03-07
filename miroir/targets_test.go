package miroir

import (
	"path/filepath"
	"testing"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/workspace"
)

func TestFlatNamesRejectsNestedPath(t *testing.T) {
	_, err := FlatNames([]string{"/tmp/ws/group/alpha"}, "/tmp/ws")
	if err == nil {
		t.Fatal("expected nested path error")
	}
}

func TestSelectTargetsByName(t *testing.T) {
	cfg := &config.Config{General: config.General{Home: "/tmp/ws"}}
	ctxs := map[string]*workspace.Context{
		filepath.Join("/tmp/ws", "alpha"): {},
		filepath.Join("/tmp/ws", "beta"):  {},
	}
	got, err := SelectTargets(cfg, ctxs, SelectOptions{Name: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "/tmp/ws/beta" {
		t.Fatalf("got %v, want [/tmp/ws/beta]", got)
	}
}

func TestSyncNamesRejectsNestedConfigName(t *testing.T) {
	cfg := &config.Config{
		General: config.General{Home: "/tmp/ws"},
		Repo: map[string]config.Repo{
			"group/alpha": {},
		},
	}
	_, err := SyncNames(cfg, SelectOptions{All: true})
	if err == nil {
		t.Fatal("expected nested config name error")
	}
}
