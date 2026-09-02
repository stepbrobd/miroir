package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
)

func setConfigFlag(t *testing.T, val string) {
	t.Helper()
	f := root.PersistentFlags().Lookup("config")
	f.Value.Set(val)
	f.Changed = val != ""
	t.Cleanup(func() {
		f.Value.Set("")
		f.Changed = false
	})
}

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
	setConfigFlag(t, "/explicit/path.toml")

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/explicit/path.toml" {
		t.Errorf("got %q, want /explicit/path.toml", got)
	}
}

func TestConfigPathEnv(t *testing.T) {
	t.Setenv("MIROIR_CONFIG", "/env/path.toml")

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/env/path.toml" {
		t.Errorf("got %q, want /env/path.toml", got)
	}
}

func TestConfigPathFlagOverEnv(t *testing.T) {
	setConfigFlag(t, "/flag.toml")
	t.Setenv("MIROIR_CONFIG", "/env.toml")

	got, err := configPath()
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

	got, err := configPath()
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

	_, err := configPath()
	if err == nil {
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
	cfg = &config.Config{
		General: config.General{Home: home},
		Repo: map[string]config.Repo{
			"live": {},
			"old":  {Archived: true},
		},
	}

	forceFlag = false
	if err := runSweep(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"live", "old", "untracked"} {
		if _, err := os.Stat(filepath.Join(home, d)); err != nil {
			t.Fatalf("dry run removed %s: %v", d, err)
		}
	}

	forceFlag = true
	t.Cleanup(func() { forceFlag = false })
	if err := runSweep(); err != nil {
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

	nameFlag = ""
	allFlag = true
	t.Cleanup(func() { allFlag = false })

	if err := resolveTargets(nil, nil); err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.General.Home != home {
		t.Fatalf("cfg not loaded: %+v", cfg)
	}
	want := filepath.Join(home, "alpha")
	if len(targets) != 1 || targets[0].Path != want {
		t.Fatalf("targets: got %v, want [%s]", targets, want)
	}
	if targets[0].Origin.URI != "git@github.com:alice/alpha" {
		t.Fatalf("origin uri: got %q", targets[0].Origin.URI)
	}
}
