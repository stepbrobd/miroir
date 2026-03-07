package workspace

import (
	"os"
	"testing"

	"ysun.co/miroir/internal/config"
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

	if _, ok := ctxs["/home/test/ws/skip"]; ok {
		t.Error("archived repo should be excluded")
	}

	active, ok := ctxs["/home/test/ws/active"]
	if !ok {
		t.Fatal("active repo not found")
	}
	if active.Branch != "master" {
		t.Errorf("active branch: got %q, want %q", active.Branch, "master")
	}

	custom, ok := ctxs["/home/test/ws/custom"]
	if !ok {
		t.Fatal("custom repo not found")
	}
	if custom.Branch != "develop" {
		t.Errorf("custom branch: got %q, want %q", custom.Branch, "develop")
	}

	if len(active.Fetch) != 1 {
		t.Errorf("active fetch remotes: got %d, want 1", len(active.Fetch))
	}
	if len(active.Push) != 1 {
		t.Errorf("active push remotes: got %d, want 1", len(active.Push))
	}
}

func TestMakeAllMultiOrigin(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cfg := &config.Config{
		General: config.General{Home: "/ws", Branch: "master"},
		Platform: map[string]config.Platform{
			"a": {Origin: true, Domain: "a.com", Access: config.SSH},
			"b": {Origin: true, Domain: "b.com", Access: config.SSH},
		},
		Repo: map[string]config.Repo{
			"r": {Visibility: config.Public},
		},
	}
	_, err := MakeAll(cfg)
	if err == nil {
		t.Fatal("expected error for multiple origins")
	}
}
