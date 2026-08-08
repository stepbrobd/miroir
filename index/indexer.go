package index

import (
	"path/filepath"

	"github.com/charmbracelet/log"
	zoekt "github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/gitindex"
	zoektindex "github.com/sourcegraph/zoekt/index"
)

// IndexRepo indexes one git repo's HEAD into the given shard directory
// name defaults to the repo directory base name
func IndexRepo(repoDir, indexDir, name string) error {
	if name == "" {
		name = filepath.Base(repoDir)
	}
	log.Info("indexing", "repo", name, "source", repoDir)

	opts := gitindex.Options{
		RepoDir:     repoDir,
		Incremental: true,
		Branches:    []string{"HEAD"},
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
