// package miroir contains high-level orchestration for miroir workflows
package miroir

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/log"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
	"ysun.co/miroir/workspace"
)

// runOptions configures a batch git operation run
type RunOptions struct {
	Targets           []string
	Contexts          map[string]*workspace.Context
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

// runGitOp runs a git operation across the selected target repositories
func RunGitOp(op gitops.Op, opts RunOptions) error {
	nr := op.Remotes(opts.PlatformCount)

	var errs []repoErr
	var errMu sync.Mutex
	addErr := func(repo, msg string) {
		errMu.Lock()
		errs = append(errs, repoErr{repo: repo, msg: msg})
		errMu.Unlock()
	}

	if nr == 0 {
		sem := make(chan struct{}, 1)
		for _, target := range opts.Targets {
			err := op.Run(gitops.Params{
				Path: target, Ctx: opts.Contexts[target], Disp: opts.Reporter,
				Slot: 0, Sem: sem, Force: opts.Force, Args: opts.Args,
			})
			if err != nil {
				name := filepath.Base(target)
				addErr(name, err.Error())
			}
		}
		opts.Reporter.Finish()
	} else {
		nrepos := len(opts.Targets)
		rc := min(opts.RepoConcurrency, nrepos)
		rcRemote := opts.RemoteConcurrency
		mc := nr
		if rcRemote > 0 {
			mc = min(rcRemote, nr)
		}

		pool := make(chan int, rc)
		for i := range rc {
			pool <- i
		}
		sem := make(chan struct{}, mc)

		var wg sync.WaitGroup
		for _, target := range opts.Targets {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				slot := <-pool
				defer func() { pool <- slot }()
				opts.Reporter.Clear(slot)

				err := op.Run(gitops.Params{
					Path: target, Ctx: opts.Contexts[target], Disp: opts.Reporter,
					Slot: slot, Sem: sem, Force: opts.Force, Args: opts.Args,
				})
				if err != nil {
					name := filepath.Base(target)
					addErr(name, err.Error())
				}
			}(target)
		}
		wg.Wait()
		opts.Reporter.Finish()
	}

	if len(errs) > 0 {
		return reportRepoErrors(errs)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// selectRunOptions builds runOptions from config targets and a reporter
func SelectRunOptions(cfg *config.Config, targets []string, ctxs map[string]*workspace.Context, reporter gitops.Reporter, force bool, args []string) RunOptions {
	return RunOptions{
		Targets:           targets,
		Contexts:          ctxs,
		PlatformCount:     len(cfg.Platform),
		RepoConcurrency:   cfg.General.Concurrency.Repo,
		RemoteConcurrency: cfg.General.Concurrency.Remote,
		Force:             force,
		Args:              args,
		Reporter:          reporter,
	}
}
