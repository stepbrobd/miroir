package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"

	"ysun.co/miroir/config"
	"ysun.co/miroir/miroir"
	"ysun.co/miroir/workspace"
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_DIRS", "")
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	_, err := configPath()
	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
}

func setupTargets(t *testing.T, home string, repos ...string) {
	t.Helper()
	cfg = &config.Config{
		General: config.General{Home: home},
	}
	ctxs = make(map[string]*workspace.Context)
	for _, r := range repos {
		ctxs[filepath.Join(home, r)] = &workspace.Context{}
	}
}

func TestSelectTargetsByName(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "alpha", "beta")
	nameFlag = "alpha"
	allFlag = false
	t.Cleanup(func() { nameFlag = "" })

	got, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
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
	t.Cleanup(func() { nameFlag = "" })

	_, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
}

func TestSelectTargetsAll(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	setupTargets(t, "/home/test/ws", "beta", "alpha")
	nameFlag = ""
	allFlag = true
	t.Cleanup(func() { allFlag = false })

	got, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err != nil {
		t.Fatal(err)
	}
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

	// macOS /var -> /private/var symlink must be resolved for path matching
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "ws", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctxs = map[string]*workspace.Context{dir: {}}
	cfg = &config.Config{General: config.General{Home: filepath.Join(tmp, "ws")}}

	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(dir)

	got, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
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

	_, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err == nil {
		t.Fatal("expected error when cwd is not a managed repo")
	}
}

func TestSelectTargetsRejectsNestedManagedRepo(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg = &config.Config{General: config.General{Home: "/home/test/ws"}}
	ctxs = map[string]*workspace.Context{
		"/home/test/ws/group/alpha": {},
	}

	_, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err == nil {
		t.Fatal("expected error for nested managed repo path")
	}
}

func TestSelectTargetsRejectsManagedRepoOutsideWorkspace(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg = &config.Config{General: config.General{Home: "/home/test/ws"}}
	ctxs = map[string]*workspace.Context{
		"/home/test/other/alpha": {},
	}

	_, err := miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err == nil {
		t.Fatal("expected error for managed repo outside workspace")
	}
}

func TestSyncNamesByName(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg = &config.Config{
		General: config.General{Home: "/home/test/ws"},
		Repo: map[string]config.Repo{
			"alpha": {},
			"beta":  {},
		},
	}
	nameFlag = "alpha"
	allFlag = false
	t.Cleanup(func() { nameFlag = "" })

	got, err := miroir.SyncNames(cfg, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("got %v, want [alpha]", got)
	}
}

func TestSyncNamesRejectsNestedRepoName(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg = &config.Config{
		General: config.General{Home: "/home/test/ws"},
		Repo: map[string]config.Repo{
			"group/alpha": {},
		},
	}

	_, err := miroir.SyncNames(cfg, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err == nil {
		t.Fatal("expected error for nested sync repo path")
	}
}
