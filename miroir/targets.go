package miroir

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/workspace"
)

type SelectOptions struct {
	Name string
	All  bool
	Cwd  string
}

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
	for _, name := range names {
		path := filepath.Join(home, name)
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{name}, nil
		}
	}
	return nil, fmt.Errorf("not a managed repository (cwd: %s)", cwd)
}

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
