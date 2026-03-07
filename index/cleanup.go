package index

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	zoektindex "github.com/sourcegraph/zoekt/index"
)

func activeManagedRepoPaths(c *Cfg) map[string]string {
	paths := make(map[string]string, len(c.Repos))
	for _, r := range c.Repos {
		paths[r.Name] = repoPath(c.Home, r.Name, c.Bare)
	}
	return paths
}

func cleanupManagedRepoDirs(c *Cfg) error {
	entries, err := os.ReadDir(c.Home)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	active := activeManagedRepoPaths(c)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, path, ok := managedRepoName(c, entry.Name())
		if !ok {
			continue
		}
		if _, ok := active[name]; ok {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		log.Info("removed stale repo", "repo", name, "path", path)
	}
	return nil
}

func managedRepoName(c *Cfg, entry string) (string, string, bool) {
	path := filepath.Join(c.Home, entry)
	if c.Bare {
		if filepath.Ext(entry) != ".git" {
			return "", "", false
		}
	} else {
		gitDir := filepath.Join(path, ".git")
		info, err := os.Stat(gitDir)
		if err != nil || !info.IsDir() {
			return "", "", false
		}
	}
	marker, ok, err := repoConfig(path, c.Env, "miroir.managed")
	if err != nil || !ok || marker != "true" {
		return "", "", false
	}
	if c.Bare {
		return entry[:len(entry)-len(".git")], path, true
	}
	return entry, path, true
}

func cleanupShards(c *Cfg, discovered []string, includeReady bool) error {
	entries, err := os.ReadDir(c.Database)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	activeByName := activeManagedRepoPaths(c)
	activeByPath := make(map[string]string, len(activeByName))
	for name, path := range activeByName {
		activeByPath[path] = name
	}
	activeIncludes := make(map[string]struct{}, len(discovered))
	for _, path := range discovered {
		activeIncludes[filepath.Clean(path)] = struct{}{}
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".zoekt" {
			continue
		}
		shard := filepath.Join(c.Database, entry.Name())
		repos, _, err := zoektindex.ReadMetadataPath(shard)
		if err != nil {
			return err
		}

		remove := false
		for _, repo := range repos {
			source := filepath.Clean(repo.Source)
			if filepath.Dir(source) == c.Home {
				expectedName, ok := activeByPath[source]
				if !ok || repo.Name != expectedName {
					remove = true
					break
				}
				continue
			}
			if !includeReady || !isIncludedSource(source, c.Include) {
				continue
			}
			if _, ok := activeIncludes[source]; !ok {
				remove = true
				break
			}
		}
		if !remove {
			continue
		}
		paths, err := zoektindex.IndexFilePaths(shard)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		log.Info("removed stale shard", "path", shard)
	}
	return nil
}

func isIncludedSource(source string, include []string) bool {
	for _, base := range include {
		rel, err := filepath.Rel(filepath.Clean(base), source)
		if err != nil {
			continue
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}
