package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/display"
	"ysun.co/miroir/internal/forge"
	"ysun.co/miroir/internal/git"
)

const syncTimeout = 30 * time.Second

func gitCmd(use, short string, op git.Op) *cobra.Command {
	return &cobra.Command{
		Use:               use,
		Short:             short,
		PersistentPreRunE: resolveTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOn(op, forceFlag, args)
		},
	}
}

func init() {
	execCmd := &cobra.Command{
		Use:               "exec [flags] -- <command> [args...]",
		Short:             "execute command in repo(s)",
		PersistentPreRunE: resolveTargets,
		Args:              cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOn(git.Exec{}, forceFlag, args)
		},
	}

	syncCmd := &cobra.Command{
		Use:               "sync",
		Short:             "sync metadata to all forges",
		PersistentPreRunE: resolveTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync()
		},
	}

	root.AddCommand(
		gitCmd("init", "initialize repo(s)", git.Init{}),
		gitCmd("fetch", "fetch from all remotes", git.Fetch{}),
		gitCmd("pull", "pull from origin", git.Pull{}),
		gitCmd("push", "push to all remotes", git.Push{}),
		execCmd,
		syncCmd,
	)
}

func runOn(op git.Op, force bool, extra []string) error {
	nr := op.Remotes(len(cfg.Platform))

	var errs []struct{ repo, msg string }
	var errMu sync.Mutex
	addErr := func(repo, msg string) {
		errMu.Lock()
		errs = append(errs, struct{ repo, msg string }{repo, msg})
		errMu.Unlock()
	}

	if nr == 0 {
		disp := display.New(1, 0, display.DefaultTheme)
		sem := make(chan struct{}, 1)
		for _, target := range targets {
			err := op.Run(git.Params{
				Path: target, Ctx: ctxs[target], Disp: disp,
				Slot: 0, Sem: sem, Force: force, Args: extra,
			})
			if err != nil {
				name := filepath.Base(target)
				addErr(name, err.Error())
				errorf("%s :: %s", name, err)
			}
		}
		disp.Finish()
	} else {
		nrepos := len(targets)
		rc := min(cfg.General.Concurrency.Repo, nrepos)
		rcRemote := cfg.General.Concurrency.Remote
		mc := nr
		if rcRemote > 0 {
			mc = min(rcRemote, nr)
		}

		disp := display.New(rc, nr, display.DefaultTheme)
		pool := make(chan int, rc)
		for i := range rc {
			pool <- i
		}
		sem := make(chan struct{}, mc)

		var wg sync.WaitGroup
		for _, target := range targets {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				slot := <-pool
				defer func() { pool <- slot }()
				disp.Clear(slot)

				err := op.Run(git.Params{
					Path: target, Ctx: ctxs[target], Disp: disp,
					Slot: slot, Sem: sem, Force: force, Args: extra,
				})
				if err != nil {
					name := filepath.Base(target)
					addErr(name, err.Error())
					disp.Error(slot, fmt.Sprintf("error: %s", err))
				}
			}(target)
		}
		wg.Wait()
		disp.Finish()
	}

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, e := range errs {
			errorf("%s :: %s", e.repo, e.msg)
		}
		return fmt.Errorf("%d operation(s) failed", len(errs))
	}
	return nil
}

func syncRepo(disp *display.Display, slot int, sem chan struct{}, name string) error {
	repo, ok := cfg.Repo[name]
	if !ok {
		disp.Repo(slot, fmt.Sprintf("%s :: sync :: no repo config", name))
		repo = config.Repo{Visibility: config.Private}
	}
	disp.Repo(slot, fmt.Sprintf("%s :: sync", name))

	pnames := slices.Sorted(maps.Keys(cfg.Platform))
	var (
		mu   sync.Mutex
		errs []string
		wg   sync.WaitGroup
	)

	for j, pname := range pnames {
		wg.Add(1)
		go func(j int, pname string, p config.Platform) {
			defer wg.Done()
			disp.Remote(slot, j, fmt.Sprintf("%s :: waiting...", pname))
			sem <- struct{}{}
			defer func() { <-sem }()

			f := config.ResolveForge(p)
			t := config.ResolveToken(pname, p)
			if f == nil {
				disp.Remote(slot, j, fmt.Sprintf("%s :: skipped", pname))
				disp.Output(slot, j, "unknown forge")
				return
			}
			if t == nil {
				disp.Remote(slot, j, fmt.Sprintf("%s :: skipped", pname))
				disp.Output(slot, j, "no token")
				return
			}

			disp.Remote(slot, j, fmt.Sprintf("%s :: syncing...", pname))
			impl, err := forge.Dispatch(*f, *t, p.Domain)
			if err != nil {
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s/%s", pname, err))
				mu.Unlock()
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
			defer cancel()
			meta := forge.Meta{
				Name:     name,
				Desc:     repo.Description,
				Vis:      repo.Visibility,
				Archived: repo.Archived,
			}
			if err := impl.Sync(ctx, p.User, meta); err != nil {
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s/%s", pname, err))
				mu.Unlock()
			} else {
				disp.Remote(slot, j, fmt.Sprintf("%s :: done", pname))
				disp.Output(slot, j, fmt.Sprintf("synced on %s", f))
			}
		}(j, pname, cfg.Platform[pname])
	}
	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func runSync() error {
	nrepos := len(targets)
	nremotes := len(cfg.Platform)
	rc := min(cfg.General.Concurrency.Repo, nrepos)
	rcRemote := cfg.General.Concurrency.Remote
	mc := nremotes
	if rcRemote > 0 {
		mc = min(rcRemote, nremotes)
	}

	disp := display.New(rc, nremotes, display.DefaultTheme)
	pool := make(chan int, rc)
	for i := range rc {
		pool <- i
	}
	sem := make(chan struct{}, mc)

	var (
		errs  []struct{ repo, msg string }
		errMu sync.Mutex
		wg    sync.WaitGroup
	)

	for _, target := range targets {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			slot := <-pool
			defer func() { pool <- slot }()
			disp.Clear(slot)

			name := filepath.Base(target)
			if err := syncRepo(disp, slot, sem, name); err != nil {
				errMu.Lock()
				errs = append(errs, struct{ repo, msg string }{name, err.Error()})
				errMu.Unlock()
			}
		}(target)
	}
	wg.Wait()
	disp.Finish()

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, e := range errs {
			errorf("%s :: %s", e.repo, e.msg)
		}
		return fmt.Errorf("%d repo(s) failed to sync", len(errs))
	}
	return nil
}
