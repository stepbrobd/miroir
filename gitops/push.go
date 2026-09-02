package gitops

import (
	"fmt"

	"ysun.co/miroir/workspace"
)

type Push struct{}

func (Push) Remotes(n int) int { return n }

func (Push) Run(p Params) error {
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: push", p.Ctx.Name))
	if err := ensureRepo(p.Ctx.Path); err != nil {
		return err
	}
	return eachRemote(p, "pushing", "push to", func(r workspace.Remote) []string {
		args := []string{"push"}
		if p.Force {
			args = append(args, "--force")
		}
		return append(args, r.GitName, p.Ctx.Branch)
	})
}
