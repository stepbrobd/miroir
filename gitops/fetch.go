package gitops

import (
	"fmt"

	"ysun.co/miroir/workspace"
)

type Fetch struct{}

func (Fetch) Remotes(n int) int { return n }

func (Fetch) Run(p Params) error {
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: fetch", repoName(p.Path)))
	if err := ensureRepo(p.Path); err != nil {
		return err
	}
	return eachRemote(p, "fetching", "fetch from", func(r workspace.Remote) []string {
		// concurrent fetches into one repo race on commit-graph
		// and auto-gc lock files
		// suppression uses config keys, not --no-auto-maintenance,
		// so any git version accepts the command
		return []string{
			"-c", "fetch.writeCommitGraph=false",
			"-c", "maintenance.auto=false",
			"-c", "gc.auto=0",
			"fetch", r.GitName,
		}
	})
}
