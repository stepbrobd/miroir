package gitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Init struct{}

func (Init) Remotes(_ int) int { return 1 }

func (Init) Run(p Params) error {
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: init", p.Ctx.Name))

	j := 0
	out := func(s string) { p.Disp.Output(p.Slot, j, s) }
	info := func(s string) { p.Disp.Remote(p.Slot, j, s) }

	info("initializing...")
	gitDir := filepath.Join(p.Ctx.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := os.MkdirAll(p.Ctx.Path, 0o755); err != nil {
			return err
		}
		if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
			"init", "--initial-branch="+p.Ctx.Branch); err != nil {
			return err
		}
	} else {
		if err := ensureRepo(p.Ctx.Path); err != nil {
			return err
		}
		dirty, err := isDirty(p.RunCtx, p.Ctx.Path, p.Ctx.Env)
		if err != nil {
			return err
		}
		if dirty && !p.Force {
			msg := "dirty working tree, use --force to override"
			p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", msg))
			return errors.New(msg)
		}
		if p.Force {
			info("cleaning untracked files...")
			if err := runQuiet(p.RunCtx, p.Ctx.Path, p.Ctx.Env,
				"clean", "-fd"); err != nil {
				return err
			}
		}
	}

	info("adding remotes...")
	setRemote := func(rname, uri string) error {
		_ = runQuiet(p.RunCtx, p.Ctx.Path, p.Ctx.Env, "remote", "remove", rname)
		return runQuiet(p.RunCtx, p.Ctx.Path, p.Ctx.Env, "remote", "add", rname, uri)
	}
	if err := setRemote("origin", p.Ctx.Origin.URI); err != nil {
		return err
	}
	for _, r := range p.Ctx.Push {
		if err := setRemote(r.Name, r.URI); err != nil {
			return err
		}
	}

	info("fetching...")
	fetchArgs := append([]string{"fetch", "--all"}, p.Args...)
	if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out, fetchArgs...); err != nil {
		return err
	}

	info("resetting...")
	if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
		"reset", "--hard", "origin/"+p.Ctx.Branch); err != nil {
		return err
	}

	info("checking out...")
	if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
		"checkout", p.Ctx.Branch); err != nil {
		return err
	}

	info("updating submodules...")
	if err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
		"submodule", "update", "--recursive", "--init"); err != nil {
		return err
	}

	info("setting upstream...")
	err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env, out,
		"branch", "--set-upstream-to=origin/"+p.Ctx.Branch, p.Ctx.Branch)
	if err != nil {
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", err))
	} else {
		info("done")
	}
	return err
}
