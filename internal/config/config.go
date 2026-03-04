package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
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
	Origin bool    `toml:"origin"`
	Domain string  `toml:"domain"`
	User   string  `toml:"user"`
	Access Access  `toml:"access"`
	Token  *string `toml:"token,omitempty"`
	Forge  *Forge  `toml:"forge,omitempty"`
}

type Repo struct {
	Description *string    `toml:"description,omitempty"`
	Visibility  Visibility `toml:"visibility"`
	Archived    bool       `toml:"archived"`
	Branch      *string    `toml:"branch,omitempty"`
}

type Config struct {
	General  General             `toml:"general"`
	Platform map[string]Platform `toml:"platform"`
	Repo     map[string]Repo     `toml:"repo"`
}

// nil if domain is not a known forge
func ForgeOfDomain(domain string) *Forge {
	d := strings.ToLower(domain)
	var f Forge
	switch {
	case d == "github.com" || strings.HasPrefix(d, "github."):
		f = Github
	case d == "gitlab.com" || strings.HasPrefix(d, "gitlab."):
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

// explicit field takes precedence over domain auto-detect
func ResolveForge(p Platform) *Forge {
	if p.Forge != nil {
		return p.Forge
	}
	return ForgeOfDomain(p.Domain)
}

// MIROIR_<NAME>_TOKEN env takes precedence over config field
func ResolveToken(name string, p Platform) *string {
	v := "MIROIR_" + strings.ToUpper(name) + "_TOKEN"
	if t, ok := os.LookupEnv(v); ok {
		return &t
	}
	return p.Token
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
	}
	if _, err := toml.Decode(s, cfg); err != nil {
		return nil, fmt.Errorf("config parse: %w", err)
	}
	return cfg, nil
}
