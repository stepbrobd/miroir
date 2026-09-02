package gitops

import (
	"errors"
	"fmt"
)

type Pull struct{}

func (Pull) Remotes(_ int) int { return 1 }

func (Pull) Run(p Params) error {
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: pull", p.Ctx.Name))
	if err := ensureRepo(p.Ctx.Path); err != nil {
		return err
	}

	j := 0
	out := func(s string) { p.Disp.Output(p.Slot, j, s) }
	info := func(s string) { p.Disp.Remote(p.Slot, j, s) }

	dirty, err := isDirty(p.RunCtx, p.Ctx.Path, p.Ctx.Env)
	if err != nil {
		return err
	}
	if !p.Force && dirty {
		msg := "dirty working tree, use --force to override"
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", msg))
		return errors.New(msg)
	}

	if p.Force {
		info("resetting...")
		if err := runQuiet(p.RunCtx, p.Ctx.Path, p.Ctx.Env,
			"reset", "--hard", "HEAD"); err != nil {
			return err
		}

		info("cleaning untracked files...")
		if err := runQuiet(p.RunCtx, p.Ctx.Path, p.Ctx.Env,
			"clean", "-fd"); err != nil {
			return err
		}
	}

	info("pulling...")
	pullArgs := append([]string{"pull", "origin", p.Ctx.Branch}, p.Args...)
	if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out, pullArgs...); err != nil {
		return err
	}

	info("updating submodules...")
	err = run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
		"submodule", "update", "--recursive", "--init")
	if err != nil {
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", err))
	} else {
		info("done")
	}
	return err
}
