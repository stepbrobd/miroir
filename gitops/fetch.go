package gitops

import (
	"fmt"
	"sync"

	"ysun.co/miroir/workspace"
)

type Fetch struct{}

func (Fetch) Remotes(n int) int { return n }

func (Fetch) Run(p Params) error {
	name := repoName(p.Path)
	p.Disp.Repo(p.Slot, fmt.Sprintf("%s :: fetch", name))
	if err := ensureRepo(p.Path); err != nil {
		return err
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

			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: fetching...", r.Name))
			// concurrent fetches into one repo race on commit-graph
			// and auto-gc lock files
			args := []string{
				"-c", "fetch.writeCommitGraph=false",
				"fetch", "--no-auto-maintenance", r.GitName,
			}
			err := run(p.RunCtx, p.Path, p.Ctx.Env, false,
				func(s string) { p.Disp.Output(p.Slot, j, s) },
				append(args, p.Args...)...)

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
			return fmt.Errorf("fetch from %s failed: %w", r.name, r.err)
		}
	}
	return nil
}
