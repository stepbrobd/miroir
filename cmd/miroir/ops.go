package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"ysun.co/miroir/internal/config"
	mirctx "ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/display"
	"ysun.co/miroir/internal/forge"
	"ysun.co/miroir/internal/git"
	"ysun.co/miroir/internal/index"
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
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync()
		},
	}

	sweepCmd := &cobra.Command{
		Use:               "sweep",
		Short:             "remove archived and untracked repos from workspace",
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweep()
		},
	}

	indexCmd := &cobra.Command{
		Use:               "index",
		Short:             "start index daemon (fetch, index, serve)",
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndex()
		},
	}

	root.AddCommand(
		gitCmd("init", "initialize repo(s)", git.Init{}),
		gitCmd("fetch", "fetch from all remotes", git.Fetch{}),
		gitCmd("pull", "pull from origin", git.Pull{}),
		gitCmd("push", "push to all remotes", git.Push{}),
		execCmd,
		syncCmd,
		sweepCmd,
		indexCmd,
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
		disp := display.New(1, 0, display.DefaultTheme, ttyOverride())
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

		disp := display.New(rc, nr, display.DefaultTheme, ttyOverride())
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
		style := display.DefaultTheme.Error
		fmt.Fprintln(os.Stderr, style.Render("error:"))
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, style.Render(fmt.Sprintf("  %s", e.repo)))
			fmt.Fprintln(os.Stderr, style.Render(fmt.Sprintf("    %s", e.msg)))
		}
		return fmt.Errorf("%d operation(s) failed", len(errs))
	}
	return nil
}

type remoteErr struct {
	remote string
	msg    string
}

func syncRepo(disp *display.Display, slot int, sem chan struct{}, name string) []remoteErr {
	repo, ok := cfg.Repo[name]
	if !ok {
		disp.Repo(slot, fmt.Sprintf("%s :: sync :: no repo config", name))
		repo = config.Repo{Visibility: config.Private}
	}
	disp.Repo(slot, fmt.Sprintf("%s :: sync", name))

	pnames := slices.Sorted(maps.Keys(cfg.Platform))
	var (
		mu   sync.Mutex
		errs []remoteErr
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
				errs = append(errs, remoteErr{pname, err.Error()})
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
				errs = append(errs, remoteErr{pname, err.Error()})
				mu.Unlock()
			} else {
				disp.Remote(slot, j, fmt.Sprintf("%s :: done", pname))
				disp.Output(slot, j, fmt.Sprintf("synced on %s", f))
			}
		}(j, pname, cfg.Platform[pname])
	}
	wg.Wait()

	return errs
}

// includes archived repos (unlike selectTargets which uses ctxs)
func syncNames() ([]string, error) {
	if nameFlag != "" {
		if _, ok := cfg.Repo[nameFlag]; !ok {
			return nil, fmt.Errorf("repo '%s' not found in config", nameFlag)
		}
		return []string{nameFlag}, nil
	}
	if allFlag {
		return slices.Sorted(maps.Keys(cfg.Repo)), nil
	}
	home, err := mirctx.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Repo)) {
		path := filepath.Join(home, name)
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{name}, nil
		}
	}
	return nil, fmt.Errorf("not a managed repository (cwd: %s)", cwd)
}

func runSync() error {
	names, err := syncNames()
	if err != nil {
		return err
	}

	nrepos := len(names)
	nremotes := len(cfg.Platform)
	rc := min(cfg.General.Concurrency.Repo, nrepos)
	rcRemote := cfg.General.Concurrency.Remote
	mc := nremotes
	if rcRemote > 0 {
		mc = min(rcRemote, nremotes)
	}

	disp := display.New(rc, nremotes, display.DefaultTheme, ttyOverride())
	pool := make(chan int, rc)
	for i := range rc {
		pool <- i
	}
	sem := make(chan struct{}, mc)

	type repoRemoteErr struct {
		repo   string
		remote string
		msg    string
	}
	var (
		errs  []repoRemoteErr
		errMu sync.Mutex
		wg    sync.WaitGroup
	)

	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			slot := <-pool
			defer func() { pool <- slot }()
			disp.Clear(slot)

			for _, re := range syncRepo(disp, slot, sem, name) {
				errMu.Lock()
				errs = append(errs, repoRemoteErr{name, re.remote, re.msg})
				errMu.Unlock()
			}
		}(name)
	}
	wg.Wait()
	disp.Finish()

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr)
		style := display.DefaultTheme.Error
		fmt.Fprintln(os.Stderr, style.Render("error:"))
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, style.Render(fmt.Sprintf("  %s :: %s", e.repo, e.remote)))
			fmt.Fprintln(os.Stderr, style.Render(fmt.Sprintf("    %s", e.msg)))
		}
		return fmt.Errorf("%d repo(s) failed to sync", len(errs))
	}
	return nil
}

func runSweep() error {
	home, err := mirctx.ExpandHome(cfg.General.Home)
	if err != nil {
		return err
	}
	home = filepath.Clean(home)

	entries, err := os.ReadDir(home)
	if err != nil {
		return fmt.Errorf("read workspace dir: %w", err)
	}

	var removals []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		repo, inConfig := cfg.Repo[name]
		if inConfig && !repo.Archived {
			continue
		}
		removals = append(removals, name)
	}

	if len(removals) == 0 {
		fmt.Println("nothing to sweep")
		return nil
	}

	if !forceFlag {
		fmt.Println("directories to remove (pass -f to actually delete):")
		for _, name := range removals {
			fmt.Printf("  %s\n", filepath.Join(home, name))
		}
		return nil
	}

	var errs []string
	for _, name := range removals {
		path := filepath.Join(home, name)
		clean := filepath.Clean(path)
		if !strings.HasPrefix(clean, home+string(filepath.Separator)) {
			errs = append(errs, fmt.Sprintf("%s: path escapes workspace root", name))
			continue
		}
		if err := os.RemoveAll(clean); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", name, err))
			continue
		}
		fmt.Printf("  removed %s\n", clean)
	}

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr)
		style := display.DefaultTheme.Error
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, style.Render(fmt.Sprintf("error: %s", e)))
		}
		return fmt.Errorf("%d removal(s) failed", len(errs))
	}
	return nil
}

func runIndex() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	c, err := index.CfgFrom(cfg)
	if err != nil {
		return err
	}
	return index.Run(ctx, c)
}
