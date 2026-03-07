package index

import (
	"path/filepath"

	"github.com/charmbracelet/log"
	zoekt "github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/gitindex"
	zoektindex "github.com/sourcegraph/zoekt/index"
)

// IndexRepo indexes a single git repo into the given shard directory
// branches defaults to ["HEAD"] if empty
func IndexRepo(repoDir, indexDir, name string, branches []string) error {
	if len(branches) == 0 {
		branches = []string{"HEAD"}
	}
	if name == "" {
		name = filepath.Base(repoDir)
	}
	log.Info("indexing", "repo", name, "source", repoDir)

	opts := gitindex.Options{
		RepoDir:     repoDir,
		Incremental: true,
		Branches:    branches,
		BuildOptions: zoektindex.Options{
			IndexDir: indexDir,
			RepositoryDescription: zoekt.Repository{
				Name: name,
			},
		},
	}
	_, err := gitindex.IndexGitRepo(opts)
	return err
}
