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

func writeAuthFile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "miroir", "auth.toml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
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

func TestAuthPathFlagOverEnv(t *testing.T) {
	t.Setenv("MIROIR_AUTH", "/env/auth.toml")

	a := &app{authFile: "/flag/auth.toml"}
	if got := a.authPath(); got != "/flag/auth.toml" {
		t.Errorf("got %q, want /flag/auth.toml", got)
	}
}

func TestAuthPathEnv(t *testing.T) {
	t.Setenv("MIROIR_AUTH", "/env/auth.toml")

	a := &app{}
	if got := a.authPath(); got != "/env/auth.toml" {
		t.Errorf("got %q, want /env/auth.toml", got)
	}
}

func TestAuthPathXDG(t *testing.T) {
	t.Setenv("MIROIR_AUTH", "")

	dir := t.TempDir()
	want := writeAuthFile(t, dir, "")

	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	a := &app{}
	if got := a.authPath(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAuthPathOptionalWhenDiscovered(t *testing.T) {
	t.Setenv("MIROIR_AUTH", "")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(tmp, "none"))
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	a := &app{}
	if got := a.authPath(); got != "" {
		t.Errorf("a missing auth file must be optional, got %q", got)
	}
}

// writeSyncConfig points MIROIR_CONFIG at a config with one platform
func writeSyncConfig(t *testing.T, dir string) {
	t.Helper()
	p := filepath.Join(dir, "config.toml")
	body := `
[platform.github]
origin = true
domain = "github.com"
user = "alice"
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIROIR_CONFIG", p)
}

func TestLoadAuthWiring(t *testing.T) {
	tmp := t.TempDir()
	writeSyncConfig(t, tmp)

	authFile := filepath.Join(tmp, "auth.toml")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(authFile, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MIROIR_AUTH", authFile)

	write("[platform.github]\ntoken = \"ghp_xxx\"\n")
	a := &app{}
	if err := a.loadAuth(nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := config.ResolveToken("github", a.auth); got == nil || *got != "ghp_xxx" {
		t.Fatalf("token: got %v", got)
	}

	// a credential for a platform the config does not define is a typo
	write("[platform.githbu]\ntoken = \"ghp_xxx\"\n")
	a = &app{}
	if err := a.loadAuth(nil, nil); err == nil {
		t.Fatal("expected an unknown platform to be rejected")
	}
	if a.auth != nil {
		t.Errorf("a rejected auth file must leave no credentials behind, got %v", a.auth)
	}

	// a named file that does not exist is an error, unlike a discovered one
	t.Setenv("MIROIR_AUTH", filepath.Join(tmp, "missing.toml"))
	a = &app{}
	if err := a.loadAuth(nil, nil); err == nil {
		t.Fatal("expected a missing named auth file to be an error")
	}
}

func TestLoadAuthWithoutFile(t *testing.T) {
	tmp := t.TempDir()
	writeSyncConfig(t, tmp)
	t.Setenv("MIROIR_AUTH", "")
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(tmp, "none"))
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	a := &app{}
	if err := a.loadAuth(nil, nil); err != nil {
		t.Fatal(err)
	}
	if a.auth != nil {
		t.Fatalf("a missing auth file leaves no credentials, got %v", a.auth)
	}
	// which leaves the env vars as the only source
	t.Setenv("MIROIR_GITHUB_TOKEN", "env-token")
	if got := config.ResolveToken("github", a.auth); got == nil || *got != "env-token" {
		t.Fatalf("env token: got %v", got)
	}
}

// the index daemon never reads a token, so a broken credential file must not
// be able to stop it
func TestIndexIgnoresMalformedAuth(t *testing.T) {
	tmp := t.TempDir()
	writeSyncConfig(t, tmp)

	authFile := filepath.Join(tmp, "auth.toml")
	if err := os.WriteFile(authFile, []byte("[platform.github\ntoken ==\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIROIR_AUTH", authFile)

	a := &app{}
	if err := a.loadConfig(nil, nil); err != nil {
		t.Fatalf("index pre-run must ignore the auth file: %v", err)
	}
	if a.auth != nil {
		t.Errorf("index must not load credentials, got %v", a.auth)
	}
	// the same file does stop sync
	if err := a.loadAuth(nil, nil); err == nil {
		t.Fatal("expected sync to reject the malformed auth file")
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
	sync, _, _ := root.Find([]string{"sync"})
	if sync.Flags().Lookup("auth") == nil {
		t.Error("sync lacks --auth")
	}
	// the index daemon never reads a token
	index, _, _ := root.Find([]string{"index"})
	if index.Flags().Lookup("auth") != nil {
		t.Error("index must not take --auth")
	}
}
