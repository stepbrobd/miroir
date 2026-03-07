package git

import (
	"fmt"
	"os"
)

type Init struct{}

func (Init) Remotes(_ int) int { return 1 }

func (Init) Run(p Params) error {
	name := repoName(p.Path)
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: init", name))

	j := 0
	out := func(s string) { p.Disp.Output(p.Slot, j, s) }
	info := func(s string) { p.Disp.Remote(p.Slot, j, s) }

	info("initializing...")
	gitDir := p.Path + "/.git"
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := os.MkdirAll(p.Path, 0o755); err != nil {
			return err
		}
		args := append([]string{"init", "--initial-branch=" + p.Ctx.Branch}, p.Args...)
		if err := run(p.Path, p.Ctx.Env, false, out, args...); err != nil {
			return err
		}
	} else {
		if err := run(p.Path, p.Ctx.Env, true, nil, "remote"); err != nil {
			return err
		}
	}

	info("adding remotes...")
	setRemote := func(rname, uri string) error {
		_ = run(p.Path, p.Ctx.Env, true, nil, "remote", "remove", rname)
		return run(p.Path, p.Ctx.Env, true, nil, "remote", "add", rname, uri)
	}
	if len(p.Ctx.Fetch) == 1 {
		if err := setRemote("origin", p.Ctx.Fetch[0].URI); err != nil {
			return err
		}
	}
	for _, r := range p.Ctx.Push {
		if err := setRemote(r.Name, r.URI); err != nil {
			return err
		}
	}

	info("fetching...")
	fetchArgs := append([]string{"fetch", "--all"}, p.Args...)
	if err := run(p.Path, p.Ctx.Env, false, out, fetchArgs...); err != nil {
		return err
	}

	info("updating submodules...")
	if err := run(p.Path, p.Ctx.Env, false, out,
		"submodule", "update", "--recursive", "--init"); err != nil {
		return err
	}

	info("resetting...")
	if err := run(p.Path, p.Ctx.Env, false, out,
		"reset", "--hard", "origin/"+p.Ctx.Branch); err != nil {
		return err
	}

	info("checking out...")
	if err := run(p.Path, p.Ctx.Env, false, out,
		"checkout", p.Ctx.Branch); err != nil {
		return err
	}

	info("setting upstream...")
	err := run(p.Path, p.Ctx.Env, false, out,
		"branch", "--set-upstream-to=origin/"+p.Ctx.Branch, p.Ctx.Branch)
	if err != nil {
		p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("error: %s", err))
	} else {
		info("done")
	}
	return err
}
