package gitops

import (
	"fmt"
	"sync"

	"ysun.co/miroir/workspace"
)

type Push struct{}

func (Push) Remotes(n int) int { return n }

func (Push) Run(p Params) error {
	name := repoName(p.Path)
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: push", name))
	if err := ensureRepo(p.Path); err != nil {
		return err
	}

	var forceArgs []string
	if p.Force {
		forceArgs = []string{"--force"}
	}

	var (
		mu      sync.Mutex
		results []struct {
			name string
			err  error
		}
		wg sync.WaitGroup
	)

	for j, r := range p.Ctx.Push {
		wg.Add(1)
		go func(j int, r workspace.Remote) {
			defer wg.Done()
			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: waiting...", r.Name))
			select {
			case p.Sem <- struct{}{}:
				defer func() { <-p.Sem }()
			case <-p.RunCtx.Done():
				return
			}

			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: pushing...", r.Name))
			args := append([]string{"push"}, forceArgs...)
			args = append(args, r.GitName, p.Ctx.Branch)
			args = append(args, p.Args...)
			err := run(p.RunCtx, p.Path, p.Ctx.Env, false,
				func(s string) { p.Disp.Output(p.Slot, j, s) },
				args...)

			if err != nil {
				p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("%s :: error", r.Name))
				p.Disp.ErrorOutput(p.Slot, j, err.Error())
			} else {
				p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: done", r.Name))
			}
			mu.Lock()
			results = append(results, struct {
				name string
				err  error
			}{r.Name, err})
			mu.Unlock()
		}(j, r)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("push to %s failed: %w", r.name, r.err)
		}
	}
	return nil
}
