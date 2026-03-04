package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	Fetch  []Remote // platforms with origin = true
	Push   []Remote // all platforms
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

// ExpandHome expands ~/ prefix to $HOME
func ExpandHome(path string) string {
	home := func() string {
		h, ok := os.LookupEnv("HOME")
		if !ok {
			fmt.Fprintf(os.Stderr, "fatal: $HOME is not set\n")
			os.Exit(1)
		}
		return h
	}
	if path == "~" {
		return home()
	}
	if strings.HasPrefix(path, "~/") {
		return home() + path[1:]
	}
	return path
}

func sortedPlatforms(m map[string]config.Platform) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func make_(env []string, platforms map[string]config.Platform, repo, branch string) *Context {
	// merge process env with config env
	base := os.Environ()
	merged := make([]string, 0, len(base)+len(env))
	merged = append(merged, base...)
	merged = append(merged, env...)

	names := sortedPlatforms(platforms)

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
	switch len(fetch) {
	case 0:
		fmt.Fprintf(os.Stderr, "warning: no platform has origin = true for %s\n", repo)
	case 1:
		// ok
	default:
		fmt.Fprintf(os.Stderr, "fatal: multiple platforms have origin = true\n")
		os.Exit(1)
	}

	var push []Remote
	for _, n := range names {
		p := platforms[n]
		push = append(push, Remote{
			Name: n,
			URI:  MakeURI(p.Access, p.Domain, p.User, repo),
		})
	}

	return &Context{Env: merged, Branch: branch, Fetch: fetch, Push: push}
}

func envSlice(m map[string]string) []string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, k+"="+v)
	}
	return s
}

// MakeAll builds (path, context) pairs for all non-archived repos
func MakeAll(cfg *config.Config) map[string]*Context {
	home := ExpandHome(cfg.General.Home)
	env := envSlice(cfg.General.Env)
	ctxs := make(map[string]*Context)
	for name, repo := range cfg.Repo {
		if repo.Archived {
			continue
		}
		path := filepath.Join(home, name)
		branch := cfg.General.Branch
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		ctxs[path] = make_(env, cfg.Platform, name, branch)
	}
	return ctxs
}
