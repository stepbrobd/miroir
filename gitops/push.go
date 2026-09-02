package gitops

import (
	"fmt"

	"ysun.co/miroir/workspace"
)

// Push pushes the branch to every remote, Tags sends every local tag along
type Push struct {
	Tags bool
}

func (Push) Remotes(n int) int { return n }

func (o Push) Run(p Params) error {
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: push", p.Ctx.Name))
	if err := ensureRepo(p.Ctx.Path); err != nil {
		return err
	}
	return eachRemote(p, "pushing", "push to", func(r workspace.Remote) []string {
		args := []string{"push"}
		if p.Force {
			args = append(args, "--force")
		}
		if o.Tags {
			args = append(args, "--tags")
		}
		return append(args, r.GitName, p.Ctx.Branch)
	})
}
