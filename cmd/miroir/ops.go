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

func gitCmd(use, short string, op gitops.Op) *cobra.Command {
	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		PersistentPreRunE: resolveTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOn(cmd.Context(), op, forceFlag, args)
		},
	}
	targetFlags(cmd)
	forceFlagOn(cmd)
	ttyFlags(cmd)
	return cmd
}

func init() {
	execCmd := &cobra.Command{
		Use:               "exec [flags] -- <command> [args...]",
		Short:             "Execute command in repo(s)",
		PersistentPreRunE: resolveTargets,
		Args:              cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOn(cmd.Context(), gitops.Exec{}, forceFlag, args)
		},
	}
	targetFlags(execCmd)
	ttyFlags(execCmd)

	syncCmd := &cobra.Command{
		Use:               "sync",
		Short:             "Sync metadata to all forges",
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context())
		},
	}
	targetFlags(syncCmd)
	ttyFlags(syncCmd)

	sweepCmd := &cobra.Command{
		Use:               "sweep",
		Short:             "Remove archived and untracked repos from workspace",
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSweep()
		},
	}
	forceFlagOn(sweepCmd)

	indexCmd := &cobra.Command{
		Use:               "index",
		Short:             "Start index daemon (fetch, index, serve)",
		PersistentPreRunE: loadConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndex(cmd.Context())
		},
	}

	root.AddCommand(
		gitCmd("init", "Initialize repo(s)", gitops.Init{}),
		gitCmd("fetch", "Fetch from all remotes", gitops.Fetch{}),
		gitCmd("pull", "Pull from origin", gitops.Pull{}),
		gitCmd("push", "Push to all remotes", gitops.Push{}),
		execCmd,
		syncCmd,
		sweepCmd,
		indexCmd,
	)
}

func runOn(ctx context.Context, op gitops.Op, force bool, extra []string) error {
	disp := display.New(min(cfg.General.Concurrency.Repo, max(1, len(targets))), op.Remotes(len(cfg.Platform)), display.DefaultTheme, ttyOverride())
	return miroir.RunGitOp(op, miroir.SelectRunOptions(ctx, cfg, targets, disp, force, extra))
}

func runSync(ctx context.Context) error {
	names, err := miroir.SyncNames(cfg, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	if err != nil {
		return err
	}
	disp := display.New(min(cfg.General.Concurrency.Repo, max(1, len(names))), len(cfg.Platform), display.DefaultTheme, ttyOverride())
	return miroir.RunSync(ctx, cfg, names, disp)
}

func runSweep() error {
	home, err := workspace.ExpandHome(cfg.General.Home)
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

func runIndex(ctx context.Context) error {
	c, err := index.CfgFrom(cfg)
	if err != nil {
		return err
	}
	return index.Run(ctx, c)
}
