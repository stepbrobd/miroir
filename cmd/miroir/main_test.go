package main

import (
	"os"
	"path/filepath"
	"testing"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/context"
)

func TestConfigPathFlag(t *testing.T) {
	cfgFlag = "/explicit/path.toml"
	defer func() { cfgFlag = "" }()

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/explicit/path.toml" {
		t.Errorf("got %q, want /explicit/path.toml", got)
	}
}

func TestConfigPathEnv(t *testing.T) {
	cfgFlag = ""
	t.Setenv("MIROIR_CONFIG", "/env/path.toml")

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/env/path.toml" {
		t.Errorf("got %q, want /env/path.toml", got)
	}
}

// flag takes precedence over env
func TestConfigPathFlagOverEnv(t *testing.T) {
	cfgFlag = "/flag.toml"
	defer func() { cfgFlag = "" }()
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
	cfgFlag = ""
	t.Setenv("MIROIR_CONFIG", "")

	dir := t.TempDir()
	xdg := filepath.Join(dir, "miroir")
	if err := os.MkdirAll(xdg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "config.toml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigPathDefaultFallback(t *testing.T) {
	cfgFlag = ""
	t.Setenv("MIROIR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "miroir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cfgDir, "config.toml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConfigPathNoConfig(t *testing.T) {
	cfgFlag = ""
	t.Setenv("MIROIR_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	_, err := configPath()
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
}

// XDG_CONFIG_HOME set should not also search ~/.config
func TestConfigPathXDGNoFallback(t *testing.T) {
	cfgFlag = ""
	t.Setenv("MIROIR_CONFIG", "")

	// put config only in ~/.config, not in XDG_CONFIG_HOME
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "miroir")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// set XDG_CONFIG_HOME to empty dir (no config there)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	_, err := configPath()
	if err == nil {
		t.Fatal("expected error: XDG_CONFIG_HOME set should not fall back to ~/.config")
	}
}

func setupTargets(t *testing.T, home string, repos ...string) {
	t.Helper()
	cfg = &config.Config{
		General: config.General{Home: home},
	}
	ctxs = make(map[string]*context.Context)
	for _, r := range repos {
		ctxs[filepath.Join(home, r)] = &context.Context{}
	}
}

func TestSelectTargetsByName(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "alpha", "beta")
	nameFlag = "alpha"
	allFlag = false
	defer func() { nameFlag = "" }()

	got, err := selectTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "/home/test/ws/alpha" {
		t.Errorf("got %v, want [/home/test/ws/alpha]", got)
	}
}

func TestSelectTargetsByNameNotFound(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "alpha")
	nameFlag = "missing"
	allFlag = false
	defer func() { nameFlag = "" }()

	_, err := selectTargets()
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestSelectTargetsAll(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "beta", "alpha")
	nameFlag = ""
	allFlag = true
	defer func() { allFlag = false }()

	got, err := selectTargets()
	if err != nil {
		t.Fatal(err)
	}
	// should be sorted
	want := []string{"/home/test/ws/alpha", "/home/test/ws/beta"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSelectTargetsByCwd(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	nameFlag = ""
	allFlag = false

	// resolve symlinks so cwd matches map keys on macOS
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "ws", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctxs = map[string]*context.Context{dir: {}}
	cfg = &config.Config{General: config.General{Home: filepath.Join(tmp, "ws")}}

	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(dir)

	got, err := selectTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != dir {
		t.Errorf("got %v, want [%s]", got, dir)
	}
}

func TestSelectTargetsNotManaged(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "alpha")
	nameFlag = ""
	allFlag = false

	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(t.TempDir())

	_, err := selectTargets()
	if err == nil {
		t.Fatal("expected error when cwd is not a managed repo")
	}
}
