package index

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	zoekt "github.com/sourcegraph/zoekt"
	zoektindex "github.com/sourcegraph/zoekt/index"
)

func activeManagedRepoPaths(c *Cfg) map[string]string {
	paths := make(map[string]string, len(c.Repos))
	for _, r := range c.Repos {
		paths[r.Name] = repoPath(c.Home, r.Name, c.Bare)
	}
	return paths
}

func activeManagedShardNames(c *Cfg) map[string]string {
	names := make(map[string]string, len(c.Repos))
	for _, r := range c.Repos {
		names[repoPath(c.Home, r.Name, c.Bare)] = r.IndexName
	}
	return names
}

func activeIndexNames(c *Cfg) map[string]struct{} {
	names := make(map[string]struct{}, len(c.Repos))
	for _, r := range c.Repos {
		names[r.IndexName] = struct{}{}
	}
	return names
}

// tempRepoDir names bootstrap temp dirs .<repo dir>.tmp-<rand>
func orphanTempPrefixes(c *Cfg) []string {
	prefixes := make([]string, 0, len(c.Repos))
	for _, r := range c.Repos {
		prefixes = append(prefixes, "."+filepath.Base(repoPath(c.Home, r.Name, c.Bare))+".tmp-")
	}
	return prefixes
}

// home may be a mixed folder, so only dirs provably owned by miroir are
// treated as crash orphans
// a temp dir for a repo since removed from config is identified by the
// miroir.managed marker its bootstrap already wrote
func orphanTempDir(c *Cfg, entry string, prefixes []string) bool {
	if !strings.HasPrefix(entry, ".") || !strings.Contains(entry, ".tmp-") {
		return false
	}
	for _, p := range prefixes {
		if strings.HasPrefix(entry, p) {
			return true
		}
	}
	marker, ok, err := repoConfig(context.Background(), filepath.Join(c.Home, entry), c.Env, managedKey)
	return err == nil && ok && marker == "true"
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
	tempPrefixes := orphanTempPrefixes(c)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if orphanTempDir(c, entry.Name(), tempPrefixes) {
			path := filepath.Join(c.Home, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			log.Info("removed orphaned temp dir", "path", path)
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
	// cleanup config reads are millisecond-scale local git calls and run
	// to completion by design, so they are not tied to the cycle context
	marker, ok, err := repoConfig(context.Background(), path, c.Env, managedKey)
	if err != nil || !ok || marker != "true" {
		return "", "", false
	}
	if c.Bare {
		return entry[:len(entry)-len(".git")], path, true
	}
	return entry, path, true
}

// removeShards deletes every shard in database whose repositories stale accepts
func removeShards(database string, stale func(repos []*zoekt.Repository) bool) error {
	entries, err := os.ReadDir(database)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".zoekt" {
			continue
		}
		shard := filepath.Join(database, entry.Name())
		repos, _, err := zoektindex.ReadMetadataPath(shard)
		if err != nil {
			return err
		}
		if !stale(repos) {
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
		names := make([]string, 0, len(repos))
		for _, repo := range repos {
			names = append(names, repo.Name)
		}
		log.Info("removed stale shard", "path", shard, "repos", names)
	}
	return nil
}

func cleanupShards(c *Cfg, discovered []string, includeReady bool) error {
	activeByPath := activeManagedShardNames(c)
	activeNames := activeIndexNames(c)
	activeIncludes := make(map[string]struct{}, len(discovered))
	for _, path := range discovered {
		activeIncludes[filepath.Clean(path)] = struct{}{}
	}
	return removeShards(c.Database, func(repos []*zoekt.Repository) bool {
		for _, repo := range repos {
			source := filepath.Clean(repo.Source)
			if filepath.Dir(source) == c.Home {
				expectedName, ok := activeByPath[source]
				if !ok || repo.Name != expectedName {
					return true
				}
				continue
			}
			if isIncludedSource(source, c.Include) {
				if _, ok := activeIncludes[source]; includeReady && !ok {
					return true
				}
				continue
			}
			// an older home setting leaves shards whose source is gone
			// but whose name still says they are ours
			if _, ok := activeNames[repo.Name]; strings.HasPrefix(repo.Name, c.Namespace) && !ok {
				return true
			}
		}
		return false
	})
}

// cleanupManagedShardsForRepo drops shards indexed from repoPath under another name
func cleanupManagedShardsForRepo(database, repoPath, expectedName string) error {
	repoPath = filepath.Clean(repoPath)
	return removeShards(database, func(repos []*zoekt.Repository) bool {
		for _, repo := range repos {
			if filepath.Clean(repo.Source) == repoPath && repo.Name != expectedName {
				return true
			}
		}
		return false
	})
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
