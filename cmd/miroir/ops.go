package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"ysun.co/miroir/display"
	"ysun.co/miroir/gitops"
	"ysun.co/miroir/index"
	"ysun.co/miroir/miroir"
	"ysun.co/miroir/workspace"
)

func (a *app) gitCmd(use, short string, op gitops.Op) *cobra.Command {
	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		PersistentPreRunE: a.resolveTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runOn(cmd.Context(), op, args)
		},
	}
	a.targetFlags(cmd)
	a.forceFlag(cmd)
	a.ttyFlags(cmd)
	return cmd
}

func (a *app) execCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "exec [flags] -- <command> [args...]",
		Short:             "Execute command in repo(s)",
		PersistentPreRunE: a.resolveTargets,
		Args:              cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runOn(cmd.Context(), gitops.Exec{}, args)
		},
	}
	a.targetFlags(cmd)
	a.ttyFlags(cmd)
	return cmd
}

func (a *app) syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "sync",
		Short:             "Sync metadata to all forges",
		PersistentPreRunE: a.loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSync(cmd.Context())
		},
	}
	a.targetFlags(cmd)
	a.ttyFlags(cmd)
	return cmd
}

func (a *app) sweepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "sweep",
		Short:             "Remove archived and untracked repos from workspace",
		PersistentPreRunE: a.loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSweep()
		},
	}
	a.forceFlag(cmd)
	return cmd
}

func (a *app) indexCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "index",
		Short:             "Start index daemon (fetch, index, serve)",
		PersistentPreRunE: a.loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runIndex(cmd.Context())
		},
	}
}

func (a *app) runOn(ctx context.Context, op gitops.Op, extra []string) error {
	disp := display.New(min(a.cfg.General.Concurrency.Repo, max(1, len(a.targets))), op.Remotes(len(a.cfg.Platform)), display.DefaultTheme, a.ttyOverride())
	return miroir.RunGitOp(op, miroir.SelectRunOptions(ctx, a.cfg, a.targets, disp, a.force, extra))
}

func (a *app) runSync(ctx context.Context) error {
	names, err := miroir.SyncNames(a.cfg, miroir.SelectOptions{Name: a.name, All: a.all})
	if err != nil {
		return err
	}
	disp := display.New(min(a.cfg.General.Concurrency.Repo, max(1, len(names))), len(a.cfg.Platform), display.DefaultTheme, a.ttyOverride())
	return miroir.RunSync(ctx, a.cfg, names, disp)
}

func (a *app) runSweep() error {
	home, err := workspace.ExpandHome(a.cfg.General.Home)
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
		repo, inConfig := a.cfg.Repo[name]
		if inConfig && !repo.Archived {
			continue
		}
		removals = append(removals, name)
	}

	if len(removals) == 0 {
		fmt.Println("nothing to sweep")
		return nil
	}

	if !a.force {
		fmt.Println("directories to remove (pass -f to actually delete):")
		for _, name := range removals {
			fmt.Printf("  %s\n", filepath.Join(home, name))
		}
		return nil
	}

	var errs []string
	for _, name := range removals {
		path := filepath.Join(home, name)
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", name, err))
			continue
		}
		fmt.Printf("  removed %s\n", path)
	}

	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return fmt.Errorf("%d removal(s) failed", len(errs))
	}
	return nil
}

func (a *app) runIndex(ctx context.Context) error {
	c, err := index.CfgFrom(a.cfg)
	if err != nil {
		return err
	}
	return index.Run(ctx, c)
}
