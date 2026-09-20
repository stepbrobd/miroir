// Package config defines miroir configuration types and parsing helpers
package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

type Access int

const (
	SSH Access = iota
	HTTPS
)

func (a *Access) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "ssh":
		*a = SSH
	case "https":
		*a = HTTPS
	default:
		return fmt.Errorf("expected either `https` or `ssh`, got %q", string(b))
	}
	return nil
}

func (a Access) String() string {
	switch a {
	case SSH:
		return "ssh"
	case HTTPS:
		return "https"
	default:
		return "unknown"
	}
}

type Forge int

const (
	Github Forge = iota
	Gitlab
	Codeberg
	Sourcehut
)

func (f *Forge) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "github":
		*f = Github
	case "gitlab":
		*f = Gitlab
	case "codeberg":
		*f = Codeberg
	case "sourcehut":
		*f = Sourcehut
	default:
		return fmt.Errorf("expected one of: github, gitlab, codeberg, sourcehut; got %q", string(b))
	}
	return nil
}

func (f Forge) String() string {
	switch f {
	case Github:
		return "github"
	case Gitlab:
		return "gitlab"
	case Codeberg:
		return "codeberg"
	case Sourcehut:
		return "sourcehut"
	default:
		return "unknown"
	}
}

type Visibility int

const (
	Private Visibility = iota
	Public
)

func (v *Visibility) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "public":
		*v = Public
	case "private":
		*v = Private
	default:
		return fmt.Errorf("expected either `public` or `private`, got %q", string(b))
	}
	return nil
}

func (v Visibility) String() string {
	switch v {
	case Public:
		return "public"
	case Private:
		return "private"
	default:
		return "unknown"
	}
}

type Concurrency struct {
	Repo   int `toml:"repo"`
	Remote int `toml:"remote"`
}

type General struct {
	Home        string            `toml:"home"`
	Branch      string            `toml:"branch"`
	Concurrency Concurrency       `toml:"concurrency"`
	Env         map[string]string `toml:"env"`
}

type Platform struct {
	Origin bool   `toml:"origin"`
	Domain string `toml:"domain"`
	User   string `toml:"user"`
	Access Access `toml:"access"`
	Forge  *Forge `toml:"forge"`
}

type Repo struct {
	Description *string    `toml:"description"`
	Visibility  Visibility `toml:"visibility"`
	Archived    bool       `toml:"archived"`
	Branch      *string    `toml:"branch"`
}

type Index struct {
	Listen   string   `toml:"listen"`
	Database string   `toml:"database"`
	Interval int      `toml:"interval"`
	Bare     bool     `toml:"bare"`
	Include  []string `toml:"include"`
}

type Config struct {
	General  General             `toml:"general"`
	Platform map[string]Platform `toml:"platform"`
	Repo     map[string]Repo     `toml:"repo"`
	Index    Index               `toml:"index"`
}

// Credential carries the auth material for one platform, every field is
// optional and a platform with none is skipped by sync
type Credential struct {
	Token *string `toml:"token"`
}

// Auth is the credential file, kept out of Config so the main config file
// cannot hold a secret and can stay in version control
// a nil *Auth carries no credentials, which is what a missing file means
type Auth struct {
	Platform map[string]Credential `toml:"platform"`
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.General.Home) == "" {
		return fmt.Errorf("general.home must not be empty")
	}
	if strings.TrimSpace(cfg.General.Branch) == "" {
		return fmt.Errorf("general.branch must not be empty")
	}
	// an empty address makes net/http listen on port 80
	if strings.TrimSpace(cfg.Index.Listen) == "" {
		return fmt.Errorf("index.listen must not be empty")
	}
	if strings.TrimSpace(cfg.Index.Database) == "" {
		return fmt.Errorf("index.database must not be empty")
	}
	for i, inc := range cfg.Index.Include {
		if strings.TrimSpace(inc) == "" {
			return fmt.Errorf("index.include[%d] must not be empty", i)
		}
	}
	if cfg.General.Concurrency.Repo <= 0 {
		return fmt.Errorf("general.concurrency.repo must be positive, got %d", cfg.General.Concurrency.Repo)
	}
	if cfg.General.Concurrency.Remote < 0 {
		return fmt.Errorf("general.concurrency.remote must be non-negative, got %d", cfg.General.Concurrency.Remote)
	}
	if cfg.Index.Interval <= 0 {
		return fmt.Errorf("index.interval must be positive, got %d", cfg.Index.Interval)
	}
	origins := 0
	tokenVars := make(map[string]string, len(cfg.Platform))
	for name, platform := range cfg.Platform {
		if platform.Origin {
			origins++
		}
		if strings.TrimSpace(platform.Domain) == "" {
			return fmt.Errorf("platform %q: domain is required", name)
		}
		tokenVar := tokenEnvVar(name)
		if prev, ok := tokenVars[tokenVar]; ok {
			return fmt.Errorf("platform %q and %q both map to %s", prev, name, tokenVar)
		}
		tokenVars[tokenVar] = name
	}
	if origins != 1 {
		return fmt.Errorf("expected exactly one platform with origin = true, got %d", origins)
	}
	// repo names become directory names and forge repo names, so they
	// must be single flat path components
	for name, repo := range cfg.Repo {
		if name == "." || name == ".." || name != filepath.Base(name) {
			return fmt.Errorf("repo name %q must be a bare directory name", name)
		}
		if repo.Branch != nil && strings.TrimSpace(*repo.Branch) == "" {
			return fmt.Errorf("repo %q: branch must not be empty", name)
		}
	}
	return nil
}

func validateAuth(a *Auth) error {
	for _, name := range slices.Sorted(maps.Keys(a.Platform)) {
		if t := a.Platform[name].Token; t != nil && strings.TrimSpace(*t) == "" {
			return fmt.Errorf("auth: platform %q: token must not be empty", name)
		}
	}
	return nil
}

// CheckAuth rejects credentials naming a platform the config does not define,
// a renamed or misspelled platform would otherwise sync unauthenticated
// cfg is required, a is not
func CheckAuth(cfg *Config, a *Auth) error {
	if a == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(a.Platform)) {
		if _, ok := cfg.Platform[name]; !ok {
			return fmt.Errorf("auth: unknown platform %q", name)
		}
	}
	return nil
}

// ForgeOfDomain returns nil if the domain is not a known forge
func ForgeOfDomain(domain string) *Forge {
	d := strings.ToLower(domain)
	var f Forge
	switch {
	case strings.HasPrefix(d, "github."):
		f = Github
	case strings.HasPrefix(d, "gitlab."):
		f = Gitlab
	case d == "codeberg.org":
		f = Codeberg
	case strings.HasSuffix(d, ".sr.ht") || d == "sr.ht":
		f = Sourcehut
	default:
		return nil
	}
	return &f
}

// ResolveForge lets an explicit field beat domain auto-detect
func ResolveForge(p Platform) *Forge {
	if p.Forge != nil {
		return p.Forge
	}
	return ForgeOfDomain(p.Domain)
}

// ResolveToken lets env var MIROIR_<NORMALIZED_NAME>_TOKEN beat the auth file
// surrounding whitespace is stripped, and a token blank after the strip is no
// credential at all
func ResolveToken(name string, a *Auth) *string {
	if v, ok := os.LookupEnv(tokenEnvVar(name)); ok {
		if t := strings.TrimSpace(v); t != "" {
			return &t
		}
	}
	if a == nil {
		return nil
	}
	if v := a.Platform[name].Token; v != nil {
		t := strings.TrimSpace(*v)
		return &t
	}
	return nil
}

func tokenEnvVar(name string) string {
	var b strings.Builder
	b.WriteString("MIROIR_")
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	b.WriteString("_TOKEN")
	return b.String()
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

func Parse(s string) (*Config, error) {
	cfg := &Config{
		General: General{
			Home:   "~/",
			Branch: "master",
			Concurrency: Concurrency{
				Repo:   1,
				Remote: 0,
			},
		},
		Index: Index{
			Listen:   ":6070",
			Database: filepath.Join(xdg.DataHome, "miroir", "index"),
			Interval: 300,
			Bare:     true,
		},
	}
	md, err := toml.Decode(s, cfg)
	if err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}
	// a misspelled key silently falling back to a zero value could make
	// sync flip live repos private, so unknown keys are fatal
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		for _, key := range undecoded {
			if len(key) == 3 && strings.EqualFold(key[0], "platform") && strings.EqualFold(key[2], "token") {
				return nil, fmt.Errorf("config parse: platform %q: token belongs in the auth file, not the config", key[1])
			}
		}
		return nil, fmt.Errorf("config parse: unknown keys %v", undecoded)
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadAuth(path string) (*Auth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseAuth(string(data))
}

func ParseAuth(s string) (*Auth, error) {
	auth := &Auth{}
	md, err := toml.Decode(s, auth)
	if err != nil {
		var pe toml.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("auth parse: line %d: invalid TOML under key %q", pe.Position.Line, pe.LastKey)
		}
		return nil, fmt.Errorf("auth parse: %w", err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("auth parse: unknown keys %v", undecoded)
	}
	if err := validateAuth(auth); err != nil {
		return nil, err
	}
	return auth, nil
}
