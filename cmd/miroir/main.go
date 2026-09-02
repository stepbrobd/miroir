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

var (
	nameFlag  string
	allFlag   bool
	forceFlag bool
	ttyFlag   bool
	noTTYFlag bool

	// set by resolveTargets before subcommand RunE
	targets []*workspace.Context
	cfg     *config.Config
)

var root = &cobra.Command{
	Use:           "miroir",
	Short:         "Repo manager wannabe?",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	root.Version = version
	root.PersistentFlags().StringP("config", "c", "", "Config file path")
}

func targetFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVarP(&nameFlag, "name", "n", "", "Target repo by name")
	f.BoolVarP(&allFlag, "all", "a", false, "Target all repos")
}

func forceFlagOn(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Force operation")
}

func ttyFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.BoolVar(&ttyFlag, "tty", false, "Force TTY output")
	f.BoolVar(&noTTYFlag, "no-tty", false, "Force plain output")
	cmd.MarkFlagsMutuallyExclusive("tty", "no-tty")
}

// --config beats MIROIR_CONFIG which beats XDG config dirs
func configPath() (string, error) {
	if p := root.PersistentFlags().Lookup("config").Value.String(); p != "" {
		return p, nil
	}
	if p := os.Getenv("MIROIR_CONFIG"); p != "" {
		return p, nil
	}
	return xdg.SearchConfigFile(filepath.Join("miroir", "config.toml"))
}

func loadConfig(cmd *cobra.Command, args []string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	cfg, err = config.Load(path)
	return err
}

func resolveTargets(cmd *cobra.Command, args []string) error {
	if err := gitops.Available(); err != nil {
		return err
	}
	if err := loadConfig(cmd, args); err != nil {
		return err
	}
	ctxs, err := workspace.MakeAll(cfg)
	if err != nil {
		return err
	}
	targets, err = miroir.SelectTargets(cfg, ctxs, miroir.SelectOptions{Name: nameFlag, All: allFlag})
	return err
}

func ttyOverride() *bool {
	if ttyFlag {
		v := true
		return &v
	}
	if noTTYFlag {
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
