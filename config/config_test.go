package config

import (
	"os"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Parse("[platform.origin]\norigin = true\ndomain = \"github.com\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.General.Home != "~/" {
		t.Errorf("home: got %q, want %q", cfg.General.Home, "~/")
	}
	if cfg.General.Branch != "master" {
		t.Errorf("branch: got %q, want %q", cfg.General.Branch, "master")
	}
	if cfg.General.Concurrency.Repo != 1 {
		t.Errorf("concurrency.repo: got %d, want 1", cfg.General.Concurrency.Repo)
	}
	if cfg.General.Concurrency.Remote != 0 {
		t.Errorf("concurrency.remote: got %d, want 0", cfg.General.Concurrency.Remote)
	}
}

func TestValidateRequiresExactlyOneOrigin(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Fatal("expected missing origin error")
	}
	_, err := Parse(`
[platform.a]
origin = true
domain = "a.com"

[platform.b]
origin = true
domain = "b.com"
`)
	if err == nil {
		t.Fatal("expected multiple origin error")
	}
}

func TestValidateConcurrencyBounds(t *testing.T) {
	_, err := Parse(`
[general.concurrency]
repo = 0

[platform.a]
origin = true
domain = "a.com"
`)
	if err == nil {
		t.Fatal("expected repo concurrency validation error")
	}

	_, err = Parse(`
[general.concurrency]
remote = -1

[platform.a]
origin = true
domain = "a.com"
`)
	if err == nil {
		t.Fatal("expected remote concurrency validation error")
	}
}

func TestValidateRejectsEmptyPaths(t *testing.T) {
	_, err := Parse(`
[general]
home = ""

[platform.a]
origin = true
domain = "a.com"
`)
	if err == nil {
		t.Fatal("expected empty home validation error")
	}

	_, err = Parse(`
[platform.a]
origin = true
domain = "a.com"

[index]
database = ""
`)
	if err == nil {
		t.Fatal("expected empty database validation error")
	}
}

func TestValidateRejectsTokenEnvVarCollision(t *testing.T) {
	_, err := Parse(`
[platform.gitlab-main]
origin = true
domain = "gitlab.com"

[platform."gitlab.main"]
origin = false
domain = "gitlab.example.com"
`)
	if err == nil {
		t.Fatal("expected token env var collision error")
	}
}

func TestSimpleConfig(t *testing.T) {
	toml := `
[general]
home = "~/Workspace"
branch = "main"

[general.concurrency]
repo = 2
remote = 3

[general.env]
FOO = "bar"

[platform.github]
origin = true
domain = "github.com"
user = "alice"
token = "tok123"

[repo.myrepo]
description = "my repo"
visibility = "public"
`
	cfg, err := Parse(toml)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.General.Home != "~/Workspace" {
		t.Errorf("home: got %q", cfg.General.Home)
	}
	if cfg.General.Branch != "main" {
		t.Errorf("branch: got %q", cfg.General.Branch)
	}
	if cfg.General.Concurrency.Repo != 2 {
		t.Errorf("concurrency.repo: got %d", cfg.General.Concurrency.Repo)
	}
	if cfg.General.Concurrency.Remote != 3 {
		t.Errorf("concurrency.remote: got %d", cfg.General.Concurrency.Remote)
	}
	if cfg.General.Env["FOO"] != "bar" {
		t.Errorf("env: got %v", cfg.General.Env)
	}

	p, ok := cfg.Platform["github"]
	if !ok {
		t.Fatal("platform github not found")
	}
	if !p.Origin {
		t.Error("origin: want true")
	}
	if p.Domain != "github.com" {
		t.Errorf("domain: got %q", p.Domain)
	}
	if p.User != "alice" {
		t.Errorf("user: got %q", p.User)
	}
	if p.Access != SSH {
		t.Errorf("access: got %v, want SSH", p.Access)
	}
	if p.Token == nil || *p.Token != "tok123" {
		t.Errorf("token: got %v", p.Token)
	}
	if p.Forge != nil {
		t.Errorf("forge: got %v, want nil", p.Forge)
	}

	r, ok := cfg.Repo["myrepo"]
	if !ok {
		t.Fatal("repo myrepo not found")
	}
	if r.Description == nil || *r.Description != "my repo" {
		t.Errorf("description: got %v", r.Description)
	}
	if r.Visibility != Public {
		t.Errorf("visibility: got %v, want Public", r.Visibility)
	}
	if r.Archived {
		t.Error("archived: want false")
	}
	if r.Branch != nil {
		t.Errorf("branch: got %v, want nil", r.Branch)
	}
}

func TestMultiPlatformConfig(t *testing.T) {
	toml := `
[general]
home = "~/"

[platform.github]
origin = true
domain = "github.com"
user = "alice"
access = "ssh"
forge = "github"

[platform.gitlab]
origin = false
domain = "gitlab.com"
user = "alice"
access = "https"
forge = "gitlab"

[platform.codeberg]
origin = false
domain = "codeberg.org"
user = "alice"

[repo.a]
visibility = "private"
branch = "develop"

[repo.b]
description = "repo b"
archived = true
`
	cfg, err := Parse(toml)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Platform) != 3 {
		t.Errorf("platforms: got %d, want 3", len(cfg.Platform))
	}
	gl := cfg.Platform["gitlab"]
	if gl.Access != HTTPS {
		t.Errorf("gitlab access: got %v, want HTTPS", gl.Access)
	}

	a := cfg.Repo["a"]
	if a.Visibility != Private {
		t.Errorf("repo a visibility: got %v, want Private", a.Visibility)
	}
	if a.Branch == nil || *a.Branch != "develop" {
		t.Errorf("repo a branch: got %v", a.Branch)
	}

	b := cfg.Repo["b"]
	if !b.Archived {
		t.Error("repo b archived: want true")
	}
}

func TestForgeOfDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   *Forge
	}{
		{"github.com", ptr(Github)},
		{"github.example.com", ptr(Github)},
		{"gitlab.com", ptr(Gitlab)},
		{"gitlab.internal.co", ptr(Gitlab)},
		{"codeberg.org", ptr(Codeberg)},
		{"git.sr.ht", ptr(Sourcehut)},
		{"sr.ht", ptr(Sourcehut)},
		{"example.com", nil},
	}
	for _, tt := range tests {
		got := ForgeOfDomain(tt.domain)
		if got == nil && tt.want == nil {
			continue
		}
		if got == nil || tt.want == nil || *got != *tt.want {
			t.Errorf("ForgeOfDomain(%q): got %v, want %v", tt.domain, got, tt.want)
		}
	}
}

func TestResolveForge(t *testing.T) {
	gh := Github
	p := Platform{Domain: "gitlab.com", Forge: &gh}
	f := ResolveForge(p)
	if f == nil || *f != Github {
		t.Errorf("explicit forge: got %v, want Github", f)
	}

	p2 := Platform{Domain: "gitlab.com"}
	f2 := ResolveForge(p2)
	if f2 == nil || *f2 != Gitlab {
		t.Errorf("auto-detect: got %v, want Gitlab", f2)
	}
}

func TestResolveToken(t *testing.T) {
	t.Setenv("MIROIR_GITHUB_TOKEN", "")
	os.Unsetenv("MIROIR_GITHUB_TOKEN")

	tok := "config-token"
	p := Platform{Token: &tok}

	got := ResolveToken("github", p)
	if got == nil || *got != "config-token" {
		t.Errorf("config token: got %v", got)
	}

	t.Setenv("MIROIR_GITHUB_TOKEN", "env-token")
	got = ResolveToken("github", p)
	if got == nil || *got != "env-token" {
		t.Errorf("env token: got %v", got)
	}
}

func TestResolveTokenNormalizesPlatformName(t *testing.T) {
	t.Setenv("MIROIR_GITLAB_MAIN_TOKEN", "env-token")

	tok := "config-token"
	p := Platform{Token: &tok}
	got := ResolveToken("gitlab-main", p)
	if got == nil || *got != "env-token" {
		t.Errorf("normalized env token: got %v", got)
	}
}

func TestTokenEnvVarNormalizesPunctuation(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "gitlab-main", want: "MIROIR_GITLAB_MAIN_TOKEN"},
		{name: "gitlab.main", want: "MIROIR_GITLAB_MAIN_TOKEN"},
		{name: "GitLab/Main", want: "MIROIR_GITLAB_MAIN_TOKEN"},
	}
	for _, tt := range tests {
		if got := tokenEnvVar(tt.name); got != tt.want {
			t.Errorf("tokenEnvVar(%q): got %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestIndexDefaults(t *testing.T) {
	cfg, err := Parse("[platform.origin]\norigin = true\ndomain = \"github.com\"\n")
	if err != nil {
		t.Fatal(err)
	}
	ix := cfg.Index
	if ix.Listen != ":6070" {
		t.Errorf("listen: got %q, want %q", ix.Listen, ":6070")
	}
	if ix.Database == "" {
		t.Error("database: want non-empty xdg default")
	}
	if ix.Interval != 300 {
		t.Errorf("interval: got %d, want 300", ix.Interval)
	}
	if !ix.Bare {
		t.Error("bare: want true")
	}
	if len(ix.Include) != 0 {
		t.Errorf("include: got %v, want empty", ix.Include)
	}
}

func TestIndexConfig(t *testing.T) {
	s := `
[platform.origin]
origin = true
domain = "github.com"

[index]
listen = ":8080"
database = "/tmp/idx"
interval = 60
bare = false
include = ["/var/lib/gitea/repos", "/opt/gitlab/repos"]
`
	cfg, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	ix := cfg.Index
	if ix.Listen != ":8080" {
		t.Errorf("listen: got %q", ix.Listen)
	}
	if ix.Database != "/tmp/idx" {
		t.Errorf("database: got %q", ix.Database)
	}
	if ix.Interval != 60 {
		t.Errorf("interval: got %d", ix.Interval)
	}
	if ix.Bare {
		t.Error("bare: want false")
	}
	if len(ix.Include) != 2 {
		t.Errorf("include: got %v", ix.Include)
	}
}

func TestAccessRoundTrip(t *testing.T) {
	cfg, err := Parse(`
[platform.test]
origin = true
domain = "test.com"
access = "https"`)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Platform["test"].Access != HTTPS {
		t.Errorf("got %v, want HTTPS", cfg.Platform["test"].Access)
	}
}

func ptr[T any](v T) *T { return &v }
