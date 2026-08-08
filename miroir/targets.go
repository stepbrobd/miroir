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

// selectOptions controls repo target selection
type SelectOptions struct {
	Name string
	All  bool
	Cwd  string
}

// resolveNames picks repo names from candidates using name, all, and cwd selection rules
func resolveNames(names []string, home string, opts SelectOptions) ([]string, error) {
	if opts.Name != "" {
		if !slices.Contains(names, opts.Name) {
			return nil, fmt.Errorf("repo '%s' not found in config", opts.Name)
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

// selectTargets resolves selected managed repo paths from config and contexts
// repo names are flat by config validation, so the context path base is the name
func SelectTargets(cfg *config.Config, ctxs map[string]*workspace.Context, opts SelectOptions) ([]string, error) {
	home, err := workspace.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ctxs))
	for path := range ctxs {
		names = append(names, filepath.Base(path))
	}
	slices.Sort(names)
	matched, err := resolveNames(names, home, opts)
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(matched))
	for i, name := range matched {
		paths[i] = filepath.Join(home, name)
	}
	return paths, nil
}

// syncNames resolves selected repo names for sync including archived config entries
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
