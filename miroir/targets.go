package miroir

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ysun.co/miroir/config"
	"ysun.co/miroir/workspace"
)

// SelectOptions controls repo target selection
type SelectOptions struct {
	Name string
	All  bool
	Cwd  string
}

// resolveNames picks repo names from candidates using name, all, and cwd selection rules
func resolveNames(names []string, home string, opts SelectOptions) ([]string, error) {
	if opts.Name != "" {
		if !slices.Contains(names, opts.Name) {
			return nil, fmt.Errorf("repo %q not found in config", opts.Name)
		}
		return []string{opts.Name}, nil
	}
	if opts.All {
		return names, nil
	}
	cwd := opts.Cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
	}
	cwd = canonicalPath(cwd)
	for _, name := range names {
		path := canonicalPath(filepath.Join(home, name))
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{name}, nil
		}
	}
	return nil, fmt.Errorf("not a managed repository (cwd: %s)", cwd)
}

// SelectTargets resolves the selected managed repositories by name, all, or cwd
func SelectTargets(cfg *config.Config, ctxs []*workspace.Context, opts SelectOptions) ([]*workspace.Context, error) {
	home, err := workspace.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ctxs))
	byName := make(map[string]*workspace.Context, len(ctxs))
	for _, c := range ctxs {
		names = append(names, c.Name)
		byName[c.Name] = c
	}
	slices.Sort(names)
	matched, err := resolveNames(names, home, opts)
	if err != nil {
		return nil, err
	}
	targets := make([]*workspace.Context, len(matched))
	for i, name := range matched {
		targets[i] = byName[name]
	}
	return targets, nil
}

// SyncNames resolves selected repo names for sync including archived config entries
func SyncNames(cfg *config.Config, opts SelectOptions) ([]string, error) {
	home, err := workspace.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	return resolveNames(slices.Sorted(maps.Keys(cfg.Repo)), home, opts)
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
