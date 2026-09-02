// Package workspace models managed repository layout and execution context assembly
package workspace

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ysun.co/miroir/config"
)

// Remote describes a named git remote URI
type Remote struct {
	Name    string
	GitName string
	URI     string
}

// Context holds derived git execution settings for one managed repository
// Path is the checkout under general.home
// Origin repeats the origin platform's push entry under its literal git name
type Context struct {
	Name   string
	Path   string
	Env    []string
	Branch string
	Origin Remote
	Push   []Remote
}

// MakeURI builds a git remote URI for the configured forge access mode
func MakeURI(access config.Access, domain, user, repo string) string {
	switch access {
	case config.SSH:
		if user == "" {
			return fmt.Sprintf("git@%s:%s", domain, repo)
		}
		return fmt.Sprintf("git@%s:%s/%s", domain, user, repo)
	default:
		if user == "" {
			return fmt.Sprintf("https://%s/%s.git", domain, repo)
		}
		return fmt.Sprintf("https://%s/%s/%s.git", domain, user, repo)
	}
}

// returns $HOME or an error if unset
func home() (string, error) {
	h, ok := os.LookupEnv("HOME")
	if !ok {
		return "", fmt.Errorf("$HOME is not set")
	}
	return h, nil
}

// ExpandHome expands a leading ~/ prefix using $HOME
func ExpandHome(path string) (string, error) {
	if path == "~" {
		return home()
	}
	if strings.HasPrefix(path, "~/") {
		h, err := home()
		if err != nil {
			return "", err
		}
		return filepath.Join(h, path[2:]), nil
	}
	return path, nil
}

// MergeEnv extends the process environment with config env entries
// a variable already set in the process environment wins over config
func MergeEnv(extra map[string]string) []string {
	base := os.Environ()
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base))
	merged := make([]string, 0, len(base)+len(extra))
	for _, item := range base {
		merged = append(merged, item)
		if name, _, ok := strings.Cut(item, "="); ok {
			seen[name] = struct{}{}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(extra)) {
		if _, ok := seen[k]; ok {
			continue
		}
		merged = append(merged, k+"="+extra[k])
	}
	return merged
}

// makeCtx assumes config validation guaranteed exactly one origin platform
func makeCtx(env []string, platforms map[string]config.Platform, name, path, branch string) *Context {
	pnames := slices.Sorted(maps.Keys(platforms))
	var origin Remote
	push := make([]Remote, 0, len(pnames))
	for _, n := range pnames {
		p := platforms[n]
		r := Remote{Name: n, GitName: n, URI: MakeURI(p.Access, p.Domain, p.User, name)}
		if p.Origin {
			r.GitName = "origin"
			origin = r
		}
		push = append(push, r)
	}
	return &Context{Name: name, Path: path, Env: env, Branch: branch, Origin: origin, Push: push}
}

// MakeAll builds execution contexts for all non-archived managed repositories sorted by name
func MakeAll(cfg *config.Config) ([]*Context, error) {
	h, err := ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	env := MergeEnv(cfg.General.Env)
	ctxs := make([]*Context, 0, len(cfg.Repo))
	for _, name := range slices.Sorted(maps.Keys(cfg.Repo)) {
		repo := cfg.Repo[name]
		if repo.Archived {
			continue
		}
		branch := cfg.General.Branch
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		ctxs = append(ctxs, makeCtx(env, cfg.Platform, name, filepath.Join(h, name), branch))
	}
	return ctxs, nil
}
