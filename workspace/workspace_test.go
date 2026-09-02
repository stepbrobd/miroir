package workspace

import (
	"os"
	"slices"
	"testing"

	"ysun.co/miroir/config"
)

func TestMakeURI(t *testing.T) {
	tests := []struct {
		access       config.Access
		domain, user string
		repo, want   string
	}{
		{config.SSH, "github.com", "alice", "repo", "git@github.com:alice/repo"},
		{config.SSH, "github.com", "", "repo", "git@github.com:repo"},
		{config.HTTPS, "github.com", "alice", "repo", "https://github.com/alice/repo.git"},
		{config.HTTPS, "github.com", "", "repo", "https://github.com/repo.git"},
	}
	for _, tt := range tests {
		got := MakeURI(tt.access, tt.domain, tt.user, tt.repo)
		if got != tt.want {
			t.Errorf("MakeURI(%v, %q, %q, %q) = %q, want %q",
				tt.access, tt.domain, tt.user, tt.repo, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	tests := []struct {
		in, want string
	}{
		{"~", "/home/test"},
		{"~/Workspace", "/home/test/Workspace"},
		{"/absolute/path", "/absolute/path"},
		{"relative", "relative"},
	}
	for _, tt := range tests {
		got, err := ExpandHome(tt.in)
		if err != nil {
			t.Fatalf("ExpandHome(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandHomeNoHOME(t *testing.T) {
	t.Setenv("HOME", "")
	os.Unsetenv("HOME")
	_, err := ExpandHome("~/test")
	if err == nil {
		t.Fatal("expected error when $HOME is unset")
	}
}

func byName(ctxs []*Context, name string) *Context {
	for _, c := range ctxs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestMakeAll(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	gh := config.Github
	tok := "tok"
	branch := "develop"
	cfg := &config.Config{
		General: config.General{
			Home:   "~/ws",
			Branch: "master",
		},
		Platform: map[string]config.Platform{
			"github": {
				Origin: true,
				Domain: "github.com",
				User:   "alice",
				Access: config.SSH,
				Forge:  &gh,
				Token:  &tok,
			},
		},
		Repo: map[string]config.Repo{
			"active": {Visibility: config.Public},
			"custom": {Visibility: config.Private, Branch: &branch},
			"skip":   {Visibility: config.Private, Archived: true},
		},
	}

	ctxs, err := MakeAll(cfg)
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, len(ctxs))
	for i, c := range ctxs {
		names[i] = c.Name
	}
	if !slices.Equal(names, []string{"active", "custom"}) {
		t.Fatalf("got %v want sorted non-archived names", names)
	}

	active := byName(ctxs, "active")
	if active.Path != "/home/test/ws/active" {
		t.Errorf("active path: got %q", active.Path)
	}
	if active.Branch != "master" {
		t.Errorf("active branch: got %q, want %q", active.Branch, "master")
	}
	if custom := byName(ctxs, "custom"); custom.Branch != "develop" {
		t.Errorf("custom branch: got %q, want %q", custom.Branch, "develop")
	}

	if len(active.Push) != 1 {
		t.Errorf("active push remotes: got %d, want 1", len(active.Push))
	}
	if active.Origin.Name != "github" || active.Origin.GitName != "origin" {
		t.Errorf("origin remote: got %+v", active.Origin)
	}
	if active.Origin.URI != "git@github.com:alice/active" {
		t.Errorf("origin uri: got %q", active.Origin.URI)
	}
	if active.Push[0] != active.Origin {
		t.Errorf("push remote: got %+v, want origin entry", active.Push[0])
	}
}

func TestMergeEnvProcessWins(t *testing.T) {
	t.Setenv("MIROIR_TEST_SET", "process")
	os.Unsetenv("MIROIR_TEST_UNSET")

	env := MergeEnv(map[string]string{
		"MIROIR_TEST_SET":   "config",
		"MIROIR_TEST_UNSET": "config",
	})
	if slices.Contains(env, "MIROIR_TEST_SET=config") {
		t.Error("config value should not override process env")
	}
	if !slices.Contains(env, "MIROIR_TEST_SET=process") {
		t.Error("process value missing")
	}
	if !slices.Contains(env, "MIROIR_TEST_UNSET=config") {
		t.Error("unset variable should come from config")
	}
}

func TestMakeAllOriginAliasPreservesDisplayName(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		General: config.General{Home: "/ws", Branch: "master"},
		Platform: map[string]config.Platform{
			"github": {Origin: true, Domain: "github.com", Access: config.SSH},
			"gitlab": {Domain: "gitlab.com", Access: config.SSH},
		},
		Repo: map[string]config.Repo{
			"r": {Visibility: config.Public},
		},
	}
	ctxs, err := MakeAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := byName(ctxs, "r")
	if len(ctx.Push) != 2 {
		t.Fatalf("push remotes: got %d, want 2", len(ctx.Push))
	}
	if ctx.Push[0].Name != "github" || ctx.Push[0].GitName != "origin" {
		t.Errorf("origin alias mismatch: %+v", ctx.Push[0])
	}
	if ctx.Push[1].Name != "gitlab" || ctx.Push[1].GitName != "gitlab" {
		t.Errorf("gitlab mismatch: %+v", ctx.Push[1])
	}
}
