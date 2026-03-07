package git

import (
	"fmt"
	"sync"

	"ysun.co/miroir/internal/context"
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

	for _, r := range p.Ctx.Push {
		wg.Add(1)
		go func(r context.Remote) {
			defer wg.Done()
			j := remoteIndex(p.Ctx, r.Name)
			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: waiting...", r.Name))
			p.Sem <- struct{}{}
			defer func() { <-p.Sem }()

			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: fetching...", r.Name))
			err := run(p.Path, p.Ctx.Env, false,
				func(s string) { p.Disp.Output(p.Slot, j, s) },
				append([]string{"fetch", r.Name}, p.Args...)...)

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
		}(r)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			return fmt.Errorf("fetch from %s failed: %s", r.name, r.err)
		}
	}
	return nil
}
