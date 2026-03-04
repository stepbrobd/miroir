package context

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ysun.co/miroir/internal/config"
)

type Remote struct {
	Name string
	URI  string
}

type Context struct {
	Env    []string
	Branch string
	Fetch  []Remote
	Push   []Remote
}

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

// returns $HOME; error if unset
func home() (string, error) {
	h, ok := os.LookupEnv("HOME")
	if !ok {
		return "", fmt.Errorf("$HOME is not set")
	}
	return h, nil
}

// expands leading ~/ to $HOME
func ExpandHome(path string) (string, error) {
	if path == "~" {
		return home()
	}
	if strings.HasPrefix(path, "~/") {
		h, err := home()
		if err != nil {
			return "", err
		}
		return h + path[1:], nil
	}
	return path, nil
}

// at most one platform may have origin = true per repo
func make_(env []string, platforms map[string]config.Platform, repo, branch string) (*Context, error) {
	base := os.Environ()
	merged := make([]string, 0, len(base)+len(env))
	merged = append(merged, base...)
	merged = append(merged, env...)

	names := slices.Sorted(maps.Keys(platforms))

	var fetch []Remote
	for _, n := range names {
		p := platforms[n]
		if p.Origin {
			fetch = append(fetch, Remote{
				Name: n,
				URI:  MakeURI(p.Access, p.Domain, p.User, repo),
			})
		}
	}
	if len(fetch) > 1 {
		return nil, fmt.Errorf("repo %q: multiple platforms have origin = true", repo)
	}
	if len(fetch) == 0 {
		fmt.Fprintf(os.Stderr, "warning: no platform has origin = true for %s\n", repo)
	}

	var push []Remote
	for _, n := range names {
		p := platforms[n]
		push = append(push, Remote{
			Name: n,
			URI:  MakeURI(p.Access, p.Domain, p.User, repo),
		})
	}

	return &Context{Env: merged, Branch: branch, Fetch: fetch, Push: push}, nil
}

func envSlice(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

func MakeAll(cfg *config.Config) (map[string]*Context, error) {
	h, err := ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	env := envSlice(cfg.General.Env)
	ctxs := make(map[string]*Context)
	for name, repo := range cfg.Repo {
		if repo.Archived {
			continue
		}
		path := filepath.Join(h, name)
		branch := cfg.General.Branch
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		ctx, err := make_(env, cfg.Platform, name, branch)
		if err != nil {
			return nil, err
		}
		ctxs[path] = ctx
	}
	return ctxs, nil
}
