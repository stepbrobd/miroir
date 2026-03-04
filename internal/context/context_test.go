package context

import (
	"testing"

	"ysun.co/miroir/internal/config"
)

func TestMakeURI(t *testing.T) {
	tests := []struct {
		access         config.Access
		domain, user   string
		repo, want     string
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
		got := ExpandHome(tt.in)
		if got != tt.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
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

	ctxs := MakeAll(cfg)

	// archived repo should be excluded
	if _, ok := ctxs["/home/test/ws/skip"]; ok {
		t.Error("archived repo should be excluded")
	}

	// active repo uses general branch
	active, ok := ctxs["/home/test/ws/active"]
	if !ok {
		t.Fatal("active repo not found")
	}
	if active.Branch != "master" {
		t.Errorf("active branch: got %q, want %q", active.Branch, "master")
	}

	// custom repo uses per-repo branch
	custom, ok := ctxs["/home/test/ws/custom"]
	if !ok {
		t.Fatal("custom repo not found")
	}
	if custom.Branch != "develop" {
		t.Errorf("custom branch: got %q, want %q", custom.Branch, "develop")
	}

	// both should have one fetch remote (origin) and one push remote
	if len(active.Fetch) != 1 {
		t.Errorf("active fetch remotes: got %d, want 1", len(active.Fetch))
	}
	if len(active.Push) != 1 {
		t.Errorf("active push remotes: got %d, want 1", len(active.Push))
	}
}

func TestMakeAllMultiOriginExits(t *testing.T) {
	// multiple origins should cause a fatal exit;
	// we can't easily test os.Exit, so skip this as an integration test
	t.Skip("multiple origin validation requires integration test")
}
