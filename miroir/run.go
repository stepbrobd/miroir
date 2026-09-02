// Package miroir contains high-level orchestration for miroir workflows
package miroir

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
	"ysun.co/miroir/workspace"
)

// RunOptions configures a batch git operation run
// Context must be non-nil
type RunOptions struct {
	Context           context.Context
	Targets           []*workspace.Context
	PlatformCount     int
	RepoConcurrency   int
	RemoteConcurrency int
	Force             bool
	Args              []string
	Reporter          gitops.Reporter
}

type repoErr struct {
	repo string
	msg  string
}

func reportRepoErrors(errs []repoErr) error {
	for _, err := range errs {
		log.Error("operation failed", "repo", err.repo, "error", err.msg)
	}
	return fmt.Errorf("%d operation(s) failed", len(errs))
}

// remoteSlots bounds concurrent remote ops per repo, 0 means all at once
func remoteSlots(limit, remotes int) int {
	if limit > 0 {
		return min(limit, remotes)
	}
	return remotes
}

// pooled runs fn for every item on a display slot, at most repos at a time
// each item gets its own semaphore so concurrency.remote bounds per repo
func pooled[T any](ctx context.Context, items []T, repos, remotes int, disp gitops.Reporter, fn func(slot int, sem chan struct{}, item T)) {
	pool := make(chan int, repos)
	for i := range repos {
		pool <- i
	}

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Go(func() {
			var slot int
			select {
			case slot = <-pool:
			case <-ctx.Done():
				return
			}
			defer func() { pool <- slot }()
			disp.Clear(slot)

			if ctx.Err() != nil {
				return
			}
			fn(slot, make(chan struct{}, remotes), item)
		})
	}
	wg.Wait()
	disp.Finish()
}

// RunGitOp runs a git operation across the selected target repositories
func RunGitOp(op gitops.Op, opts RunOptions) error {
	ctx := opts.Context
	nr := op.Remotes(opts.PlatformCount)

	var errs []repoErr
	var errMu sync.Mutex
	runOne := func(slot int, sem chan struct{}, target *workspace.Context) {
		err := op.Run(gitops.Params{
			RunCtx: ctx, Ctx: target, Disp: opts.Reporter,
			Slot: slot, Sem: sem, Force: opts.Force, Args: opts.Args,
		})
		if err != nil && ctx.Err() == nil {
			errMu.Lock()
			errs = append(errs, repoErr{repo: target.Name, msg: err.Error()})
			errMu.Unlock()
		}
	}

	if nr == 0 {
		sem := make(chan struct{}, 1)
		for _, target := range opts.Targets {
			if ctx.Err() != nil {
				break
			}
			runOne(0, sem, target)
		}
		opts.Reporter.Finish()
	} else {
		rc := min(opts.RepoConcurrency, len(opts.Targets))
		pooled(ctx, opts.Targets, rc, remoteSlots(opts.RemoteConcurrency, nr), opts.Reporter, runOne)
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if len(errs) > 0 {
		return reportRepoErrors(errs)
	}
	return nil
}

// SelectRunOptions builds RunOptions from config targets and a reporter
func SelectRunOptions(ctx context.Context, cfg *config.Config, targets []*workspace.Context, reporter gitops.Reporter, force bool, args []string) RunOptions {
	return RunOptions{
		Context:           ctx,
		Targets:           targets,
		PlatformCount:     len(cfg.Platform),
		RepoConcurrency:   cfg.General.Concurrency.Repo,
		RemoteConcurrency: cfg.General.Concurrency.Remote,
		Force:             force,
		Args:              args,
		Reporter:          reporter,
	}
}
