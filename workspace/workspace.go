// Package workspace models managed repository layout and execution context assembly.
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

// Remote describes a named git remote URI.
type Remote struct {
	Name    string
	GitName string
	URI     string
}

// Context contains derived git execution settings for one managed repository.
type Context struct {
	Env    []string
	Branch string
	Fetch  []Remote
	Push   []Remote
}

// MakeURI builds a git remote URI for the configured forge access mode.
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
// ExpandHome expands a leading ~/ prefix using $HOME.
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

// at most one platform may have origin = true per repo
func makeCtx(env []string, platforms map[string]config.Platform, repo, branch string) (*Context, error) {
	base := os.Environ()
	merged := make([]string, 0, len(base)+len(env))
	merged = append(merged, env...)
	merged = append(merged, base...)

	names := slices.Sorted(maps.Keys(platforms))
	originName := ""
	for _, n := range names {
		if platforms[n].Origin {
			originName = n
			break
		}
	}

	var fetch []Remote
	for _, n := range names {
		p := platforms[n]
		if p.Origin {
			fetch = append(fetch, Remote{
				Name:    n,
				GitName: "origin",
				URI:     MakeURI(p.Access, p.Domain, p.User, repo),
			})
		}
	}

	var push []Remote
	for _, n := range names {
		p := platforms[n]
		gitName := n
		if n == originName {
			gitName = "origin"
		}
		push = append(push, Remote{
			Name:    n,
			GitName: gitName,
			URI:     MakeURI(p.Access, p.Domain, p.User, repo),
		})
	}

	return &Context{Env: merged, Branch: branch, Fetch: fetch, Push: push}, nil
}

func envSlice(m map[string]string) []string {
	keys := slices.Sorted(maps.Keys(m))
	s := make([]string, 0, len(m))
	for _, k := range keys {
		s = append(s, k+"="+m[k])
	}
	return s
}

// MakeAll builds execution contexts for all non-archived managed repositories.
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
		ctx, err := makeCtx(env, cfg.Platform, name, branch)
		if err != nil {
			return nil, err
		}
		ctxs[path] = ctx
	}
	return ctxs, nil
}
