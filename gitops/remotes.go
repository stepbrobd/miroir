package gitops

import (
	"errors"
	"fmt"
	"sync"

	"ysun.co/miroir/workspace"
)

// eachRemote runs one git command per push remote under the remote semaphore
// verb names the in-progress state and fail prefixes the remote name in errors
// every failed remote is reported, not only the first
func eachRemote(p Params, verb, fail string, args func(r workspace.Remote) []string) error {
	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)
	for j, r := range p.Ctx.Push {
		wg.Go(func() {
			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: waiting...", r.Name))
			select {
			case p.Sem <- struct{}{}:
				defer func() { <-p.Sem }()
			case <-p.RunCtx.Done():
				return
			}

			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: %s...", r.Name, verb))
			err := run(p.RunCtx, p.Ctx.Path, p.Ctx.Env,
				func(s string) { p.Disp.Output(p.Slot, j, s) },
				append(args(r), p.Args...)...)
			if err != nil {
				p.Disp.ErrorRemote(p.Slot, j, fmt.Sprintf("%s :: error", r.Name))
				p.Disp.ErrorOutput(p.Slot, j, err.Error())
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s %s failed: %w", fail, r.Name, err))
				mu.Unlock()
				return
			}
			p.Disp.Remote(p.Slot, j, fmt.Sprintf("%s :: done", r.Name))
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}
