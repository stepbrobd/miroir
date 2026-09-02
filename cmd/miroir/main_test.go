package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
)

func writeConfigFile(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "miroir", "config.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigPathFlag(t *testing.T) {
	a := &app{config: "/explicit/path.toml"}
	got, err := a.configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/explicit/path.toml" {
		t.Errorf("got %q, want /explicit/path.toml", got)
	}
}

func TestConfigPathEnv(t *testing.T) {
	t.Setenv("MIROIR_CONFIG", "/env/path.toml")

	a := &app{}
	got, err := a.configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/env/path.toml" {
		t.Errorf("got %q, want /env/path.toml", got)
	}
}

func TestConfigPathFlagOverEnv(t *testing.T) {
	t.Setenv("MIROIR_CONFIG", "/env.toml")

	a := &app{config: "/flag.toml"}
	got, err := a.configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/flag.toml" {
		t.Errorf("got %q, want /flag.toml", got)
	}
}

func TestConfigPathXDG(t *testing.T) {
	t.Setenv("MIROIR_CONFIG", "")

	dir := t.TempDir()
	want := writeConfigFile(t, dir)

	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	a := &app{}
	got, err := a.configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigPathNoConfig(t *testing.T) {
	t.Setenv("MIROIR_CONFIG", "")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_CONFIG_DIRS", "")
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	a := &app{}
	if _, err := a.configPath(); err == nil {
		t.Fatal("expected error when no config file exists")
	}
}

func TestRunSweep(t *testing.T) {
	home := t.TempDir()
	for _, d := range []string{"live", "old", "untracked"} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a := &app{cfg: &config.Config{
		General: config.General{Home: home},
		Repo: map[string]config.Repo{
			"live": {},
			"old":  {Archived: true},
		},
	}}

	if err := a.runSweep(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"live", "old", "untracked"} {
		if _, err := os.Stat(filepath.Join(home, d)); err != nil {
			t.Fatalf("dry run removed %s: %v", d, err)
		}
	}

	a.force = true
	if err := a.runSweep(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "live")); err != nil {
		t.Fatalf("live repo should be kept: %v", err)
	}
	for _, d := range []string{"old", "untracked"} {
		if _, err := os.Stat(filepath.Join(home, d)); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed, got %v", d, err)
		}
	}
}

func TestResolveTargetsWiring(t *testing.T) {
	if err := gitops.Available(); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	home := filepath.Join(tmp, "ws")
	cfgFile := filepath.Join(tmp, "config.toml")
	toml := `
[general]
home = "` + home + `"

[platform.github]
origin = true
domain = "github.com"
user = "alice"

[repo.alpha]
`
	if err := os.WriteFile(cfgFile, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIROIR_CONFIG", cfgFile)

	a := &app{all: true}
	if err := a.resolveTargets(nil, nil); err != nil {
		t.Fatal(err)
	}
	if a.cfg == nil || a.cfg.General.Home != home {
		t.Fatalf("cfg not loaded: %+v", a.cfg)
	}
	want := filepath.Join(home, "alpha")
	if len(a.targets) != 1 || a.targets[0].Path != want {
		t.Fatalf("targets: got %v, want [%s]", a.targets, want)
	}
	if a.targets[0].Origin.URI != "git@github.com:alice/alpha" {
		t.Fatalf("origin uri: got %q", a.targets[0].Origin.URI)
	}
}

func TestRootCommandsAndFlags(t *testing.T) {
	root := newRoot()
	for _, name := range []string{"init", "fetch", "pull", "push", "exec", "sync", "sweep", "index", "completion"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd.Name() != name {
			t.Fatalf("missing command %s: %v", name, err)
		}
	}
	push, _, _ := root.Find([]string{"push"})
	for _, flag := range []string{"name", "all", "force", "tags", "tty", "no-tty"} {
		if push.Flags().Lookup(flag) == nil {
			t.Errorf("push lacks --%s", flag)
		}
	}
	fetch, _, _ := root.Find([]string{"fetch"})
	if fetch.Flags().Lookup("tags") != nil {
		t.Error("fetch must not take --tags")
	}
	sweep, _, _ := root.Find([]string{"sweep"})
	if sweep.Flags().Lookup("all") != nil {
		t.Error("sweep must not take --all")
	}
}
