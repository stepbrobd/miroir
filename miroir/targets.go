// package miroir contains high-level orchestration for miroir workflows
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

// flatNames validates that paths are direct children of home and returns their names
func FlatNames(paths []string, home string) ([]string, error) {
	names := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return nil, fmt.Errorf("repo path %q: %w", path, err)
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is outside workspace %q", path, home)
		}
		if strings.Contains(rel, string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is not flat under workspace %q", path, home)
		}
		if _, ok := seen[rel]; ok {
			return nil, fmt.Errorf("duplicate repo name %q under workspace %q", rel, home)
		}
		seen[rel] = struct{}{}
		names = append(names, rel)
	}
	slices.Sort(names)
	return names, nil
}

// resolveNames picks repo names from candidates using name all and cwd selection rules
func ResolveNames(names []string, home string, opts SelectOptions) ([]string, error) {
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
func SelectTargets(cfg *config.Config, ctxs map[string]*workspace.Context, opts SelectOptions) ([]string, error) {
	home, err := workspace.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	names, err := FlatNames(slices.Collect(maps.Keys(ctxs)), home)
	if err != nil {
		return nil, err
	}
	matched, err := ResolveNames(names, home, opts)
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
	names := slices.Sorted(maps.Keys(cfg.Repo))
	for _, name := range names {
		path := filepath.Join(home, name)
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return nil, fmt.Errorf("repo path %q: %w", path, err)
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is outside workspace %q", path, home)
		}
		if strings.Contains(rel, string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is not flat under workspace %q", path, home)
		}
	}
	return ResolveNames(names, home, opts)
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
