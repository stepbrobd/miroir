package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
	"ysun.co/miroir/miroir"
	"ysun.co/miroir/workspace"
)

var version = "dev"

// app holds the flag values and what the pre-run hooks resolve from them
type app struct {
	config string
	name   string
	all    bool
	force  bool
	tags   bool
	tty    bool
	noTTY  bool

	cfg     *config.Config
	targets []*workspace.Context
}

func newRoot() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:           "miroir",
		Short:         "Repo manager wannabe?",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&a.config, "config", "c", "", "Config file path")
	root.AddCommand(
		a.gitCmd("init", "Initialize repo(s)", gitops.Init{}),
		a.gitCmd("fetch", "Fetch from all remotes", gitops.Fetch{}),
		a.gitCmd("pull", "Pull from origin", gitops.Pull{}),
		a.pushCmd(),
		a.execCmd(),
		a.syncCmd(),
		a.sweepCmd(),
		a.indexCmd(),
		completionCmd(root),
	)
	return root
}

func (a *app) targetFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&a.name, "name", "n", "", "Target repo by name")
	f.BoolVarP(&a.all, "all", "a", false, "Target all repos")
}

func (a *app) forceFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&a.force, "force", "f", false, "Force operation")
}

func (a *app) ttyFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.BoolVar(&a.tty, "tty", false, "Force TTY output")
	f.BoolVar(&a.noTTY, "no-tty", false, "Force plain output")
	cmd.MarkFlagsMutuallyExclusive("tty", "no-tty")
}

// --config beats MIROIR_CONFIG which beats XDG config dirs
func (a *app) configPath() (string, error) {
	if a.config != "" {
		return a.config, nil
	}
	if p := os.Getenv("MIROIR_CONFIG"); p != "" {
		return p, nil
	}
	return xdg.SearchConfigFile(filepath.Join("miroir", "config.toml"))
}

func (a *app) loadConfig(*cobra.Command, []string) error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	a.cfg, err = config.Load(path)
	return err
}

func (a *app) resolveTargets(cmd *cobra.Command, args []string) error {
	if err := gitops.Available(); err != nil {
		return err
	}
	if err := a.loadConfig(cmd, args); err != nil {
		return err
	}
	ctxs, err := workspace.MakeAll(a.cfg)
	if err != nil {
		return err
	}
	a.targets, err = miroir.SelectTargets(a.cfg, ctxs, miroir.SelectOptions{Name: a.name, All: a.all})
	return err
}

func (a *app) ttyOverride() *bool {
	if a.tty {
		v := true
		return &v
	}
	if a.noTTY {
		v := false
		return &v
	}
	return nil
}

func normalizeHelpText(cmd *cobra.Command) {
	cmd.InitDefaultHelpFlag()
	if flag := cmd.Flags().Lookup("help"); flag != nil {
		flag.Usage = "Help for " + cmd.CommandPath()
	}
	for _, child := range cmd.Commands() {
		normalizeHelpText(child)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := newRoot()
	normalizeHelpText(root)
	root.InitDefaultVersionFlag()
	if flag := root.Flags().Lookup("version"); flag != nil {
		flag.Usage = "Version for " + root.CommandPath()
	}
	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		log.Fatal(err)
	}
}
