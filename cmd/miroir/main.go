package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
	targets []string
	ctxs    map[string]*workspace.Context
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

	f := root.PersistentFlags()
	f.StringP("config", "c", "", "Config file path")
	f.StringVarP(&nameFlag, "name", "n", "", "Target repo by name")
	f.BoolVarP(&allFlag, "all", "a", false, "Target all repos")
	f.BoolVarP(&forceFlag, "force", "f", false, "Force operation")
	f.BoolVar(&ttyFlag, "tty", false, "Force TTY output")
	f.BoolVar(&noTTYFlag, "no-tty", false, "Force plain output")
	root.MarkFlagsMutuallyExclusive("tty", "no-tty")
}

// --config beats MIROIR_CONFIG which beats XDG config dirs
func configPath() (string, error) {
	v := viper.New()
	v.SetEnvPrefix("MIROIR")
	v.BindEnv("config")
	v.BindPFlag("config", root.PersistentFlags().Lookup("config"))

	if p := v.GetString("config"); p != "" {
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
	var err error
	ctxs, err = workspace.MakeAll(cfg)
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
	normalizeHelpText(root)
	root.InitDefaultVersionFlag()
	if flag := root.Flags().Lookup("version"); flag != nil {
		flag.Usage = "Version for " + root.CommandPath()
	}
	if err := root.Execute(); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		log.Fatal(err)
	}
}
