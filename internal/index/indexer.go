package index

import (
	"path/filepath"

	"github.com/charmbracelet/log"
	zoekt "github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/gitindex"
	zoektindex "github.com/sourcegraph/zoekt/index"
)

// IndexRepo indexes a single git repo into the given shard directory.
// branches defaults to ["HEAD"] if empty.
func IndexRepo(repoDir, indexDir string, branches []string) error {
	if len(branches) == 0 {
		branches = []string{"HEAD"}
	}
	log.Info("indexing", "repo", repoDir)

	// use dir basename as fallback name for repos without remotes
	name := filepath.Base(repoDir)

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
