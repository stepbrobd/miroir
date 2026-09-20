package config

import (
	"os"
	"path/filepath"
	"strings"
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
		{"github.com", new(Github)},
		{"github.example.com", new(Github)},
		{"gitlab.com", new(Gitlab)},
		{"gitlab.internal.co", new(Gitlab)},
		{"codeberg.org", new(Codeberg)},
		{"git.sr.ht", new(Sourcehut)},
		{"sr.ht", new(Sourcehut)},
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

	tok := "auth-token"
	auth := &Auth{Platform: map[string]Credential{"github": {Token: &tok}}}

	got := ResolveToken("github", auth)
	if got == nil || *got != "auth-token" {
		t.Errorf("auth token: got %v", got)
	}

	t.Setenv("MIROIR_GITHUB_TOKEN", "env-token")
	got = ResolveToken("github", auth)
	if got == nil || *got != "env-token" {
		t.Errorf("env token: got %v", got)
	}
}

func TestResolveTokenNormalizesPlatformName(t *testing.T) {
	t.Setenv("MIROIR_GITLAB_MAIN_TOKEN", "env-token")

	tok := "auth-token"
	auth := &Auth{Platform: map[string]Credential{"gitlab-main": {Token: &tok}}}
	got := ResolveToken("gitlab-main", auth)
	if got == nil || *got != "env-token" {
		t.Errorf("normalized env token: got %v", got)
	}
}

// the auth file is optional, and so is every entry in it
func TestResolveTokenWithoutCredential(t *testing.T) {
	t.Setenv("MIROIR_GITHUB_TOKEN", "")
	os.Unsetenv("MIROIR_GITHUB_TOKEN")

	if got := ResolveToken("github", nil); got != nil {
		t.Errorf("nil auth: got %v, want nil", got)
	}
	if got := ResolveToken("github", &Auth{}); got != nil {
		t.Errorf("empty auth: got %v, want nil", got)
	}
	auth := &Auth{Platform: map[string]Credential{"github": {}, "gitlab": {}}}
	if got := ResolveToken("github", auth); got != nil {
		t.Errorf("entry without a token: got %v, want nil", got)
	}
}

func TestResolveTokenIgnoresBlankEnvVar(t *testing.T) {
	t.Setenv("MIROIR_GITHUB_TOKEN", "")

	tok := "auth-token"
	auth := &Auth{Platform: map[string]Credential{"github": {Token: &tok}}}
	got := ResolveToken("github", auth)
	if got == nil || *got != "auth-token" {
		t.Errorf("a blank env var must not mask the auth file: got %v", got)
	}
}

func TestResolveTokenTrimsWhitespace(t *testing.T) {
	t.Setenv("MIROIR_GITHUB_TOKEN", "")
	os.Unsetenv("MIROIR_GITHUB_TOKEN")

	padded := "  ghp_xxx\n"
	auth := &Auth{Platform: map[string]Credential{"github": {Token: &padded}}}
	if got := ResolveToken("github", auth); got == nil || *got != "ghp_xxx" {
		t.Errorf("auth file: got %v", got)
	}

	t.Setenv("MIROIR_GITHUB_TOKEN", "  ghp_env\t")
	if got := ResolveToken("github", auth); got == nil || *got != "ghp_env" {
		t.Errorf("env var: got %v", got)
	}
}

func TestParseRejectsTokenInConfig(t *testing.T) {
	// the decoder matches field names case insensitively, so every casing
	// has to reach the same hint
	for _, key := range []string{"token", "Token", "TOKEN"} {
		_, err := Parse("[platform.github]\norigin = true\ndomain = \"github.com\"\n" + key + " = \"tok123\"\n")
		if err == nil {
			t.Fatalf("%s: expected a token in the config to be rejected", key)
		}
		if !strings.Contains(err.Error(), "auth file") {
			t.Errorf("%s: error should name the auth file: %v", key, err)
		}
		if strings.Contains(err.Error(), "tok123") {
			t.Errorf("%s: error must not echo the token: %v", key, err)
		}
	}
}

// the parser quotes the offending value back, and in this file that is a secret
func TestParseAuthErrorOmitsTokenValue(t *testing.T) {
	_, err := ParseAuth("[platform.github]\ntoken = ghpSECRETVALUE\n")
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if strings.Contains(err.Error(), "ghpSECRETVALUE") {
		t.Errorf("error must not echo the token: %v", err)
	}
	if !strings.Contains(err.Error(), "platform.github.token") {
		t.Errorf("error should name the key: %v", err)
	}
}

func TestParseAuth(t *testing.T) {
	auth, err := ParseAuth(`
[platform.github]
token = "ghp_xxx"

[platform.gitlab]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveToken("github", auth); got == nil || *got != "ghp_xxx" {
		t.Errorf("github token: got %v", got)
	}
	gl, ok := auth.Platform["gitlab"]
	if !ok || gl.Token != nil {
		t.Errorf("an entry may carry no token: got %v", gl.Token)
	}
}

func TestParseAuthEmpty(t *testing.T) {
	auth, err := ParseAuth("")
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveToken("github", auth); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestParseAuthRejectsUnknownKeys(t *testing.T) {
	// only auth material belongs here, a platform field is a mistake
	if _, err := ParseAuth("[platform.github]\ndomain = \"github.com\"\n"); err == nil {
		t.Fatal("expected a platform field to be rejected")
	}
	if _, err := ParseAuth("[platform.github]\ntokne = \"x\"\n"); err == nil {
		t.Fatal("expected a misspelled key to be rejected")
	}
	if _, err := ParseAuth("[general]\nhome = \"~/\"\n"); err == nil {
		t.Fatal("expected a config table to be rejected")
	}
}

func TestParseAuthRejectsBlankToken(t *testing.T) {
	if _, err := ParseAuth("[platform.github]\ntoken = \"  \"\n"); err == nil {
		t.Fatal("expected a blank token to be rejected")
	}
}

func TestCheckAuth(t *testing.T) {
	cfg, err := Parse("[platform.github]\norigin = true\ndomain = \"github.com\"\n")
	if err != nil {
		t.Fatal(err)
	}
	tok := "t"
	if err := CheckAuth(cfg, nil); err != nil {
		t.Errorf("nil auth: %v", err)
	}
	if err := CheckAuth(cfg, &Auth{}); err != nil {
		t.Errorf("empty auth: %v", err)
	}
	ok := &Auth{Platform: map[string]Credential{"github": {Token: &tok}}}
	if err := CheckAuth(cfg, ok); err != nil {
		t.Errorf("configured platform: %v", err)
	}
	// a renamed or misspelled platform would otherwise sync unauthenticated
	bad := &Auth{Platform: map[string]Credential{"githbu": {Token: &tok}}}
	if err := CheckAuth(cfg, bad); err == nil {
		t.Fatal("expected an unknown platform to be rejected")
	}
}

func TestLoadAuth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.toml")
	if err := os.WriteFile(path, []byte("[platform.github]\ntoken = \"ghp_xxx\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := LoadAuth(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveToken("github", auth); got == nil || *got != "ghp_xxx" {
		t.Errorf("token: got %v", got)
	}

	if _, err := LoadAuth(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Fatal("expected an error for a named file that does not exist")
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

func TestParseRejectsUnknownKeys(t *testing.T) {
	tests := []string{
		"[platform.a]\norigin = true\ndomain = \"a.com\"\n\n[repo.x]\nvisiblity = \"public\"\n",
		"[platform.a]\norigin = true\ndomain = \"a.com\"\naccess_mode = \"ssh\"\n",
		"[genral]\nhome = \"~/\"\n\n[platform.a]\norigin = true\ndomain = \"a.com\"\n",
	}
	for _, s := range tests {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected unknown key error for %q", s)
		}
	}
}

func TestValidateRejectsNonFlatRepoNames(t *testing.T) {
	for _, name := range []string{"a/b", "a/.", "./a", "..", "."} {
		s := "[platform.a]\norigin = true\ndomain = \"a.com\"\n\n[repo.\"" + name + "\"]\nvisibility = \"private\"\n"
		if _, err := Parse(s); err == nil {
			t.Errorf("expected repo name error for %q", name)
		}
	}
}

func TestValidateRejectsNonPositiveInterval(t *testing.T) {
	for _, interval := range []string{"0", "-1"} {
		_, err := Parse(`
[platform.a]
origin = true
domain = "a.com"

[index]
interval = ` + interval + `
`)
		if err == nil {
			t.Fatalf("expected interval validation error for %s", interval)
		}
	}
}

func TestValidateRequiresDomain(t *testing.T) {
	_, err := Parse(`
[platform.a]
origin = true
domain = "  "
`)
	if err == nil {
		t.Fatal("expected domain required error")
	}
}

func TestUnmarshalTextRejectsUnknownValues(t *testing.T) {
	tests := []string{
		"[platform.a]\norigin = true\ndomain = \"a.com\"\naccess = \"telnet\"\n",
		"[platform.a]\norigin = true\ndomain = \"a.com\"\nforge = \"bitbucket\"\n",
		"[platform.a]\norigin = true\ndomain = \"a.com\"\n\n[repo.x]\nvisibility = \"internal\"\n",
	}
	for _, s := range tests {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected enum decode error for %q", s)
		}
	}
}

func TestValidateRejectsEmptyListenBranchAndInclude(t *testing.T) {
	origin := "[platform.a]\norigin = true\ndomain = \"a.com\"\n"
	tests := []string{
		origin + "\n[general]\nbranch = \"\"\n",
		origin + "\n[index]\nlisten = \" \"\n",
		origin + "\n[index]\ninclude = [\"/srv/repos\", \"\"]\n",
		origin + "\n[repo.x]\nbranch = \"\"\n",
	}
	for _, s := range tests {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected validation error for %q", s)
		}
	}
}
