package git

import (
	"errors"
	"fmt"
)

type Pull struct{}

func (Pull) Remotes(_ int) int { return 1 }

func (Pull) Run(p Params) error {
	name := repoName(p.Path)
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: pull", name))
	if err := ensureRepo(p.Path); err != nil {
		return err
	}

	j := 0
	out := func(s string) { p.Disp.Output(p.Slot, j, s) }
	info := func(s string) { p.Disp.Remote(p.Slot, j, s) }

	if !p.Force && isDirty(p.Path, p.Ctx.Env) {
		msg := "dirty working tree, use --force to override"
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", msg))
		return errors.New(msg)
	}

	if p.Force {
		info("resetting...")
		if err := run(p.Path, p.Ctx.Env, true, nil,
			"reset", "--hard", "HEAD"); err != nil {
			return err
		}
	}

	info("pulling...")
	pullArgs := append([]string{"pull", "origin", p.Ctx.Branch}, p.Args...)
	if err := run(p.Path, p.Ctx.Env, false, out, pullArgs...); err != nil {
		return err
	}

	info("updating submodules...")
	err := run(p.Path, p.Ctx.Env, false, out,
		"submodule", "update", "--recursive", "--init")
	if err != nil {
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", err))
	} else {
		info("done")
	}
	return err
}
