package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/git"
)

var version = "dev"

var (
	nameFlag  string
	allFlag   bool
	forceFlag bool

	// set by resolveTargets; must be populated before subcommand RunE
	targets []string
	ctxs    map[string]*context.Context
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
	f.StringP("config", "c", "", "config file path")
	f.StringVarP(&nameFlag, "name", "n", "", "target repo by name")
	f.BoolVarP(&allFlag, "all", "a", false, "target all repos")
	f.BoolVarP(&forceFlag, "force", "f", false, "force operation")
}

// priority: --config flag > MIROIR_CONFIG env > XDG config dirs
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

func resolveTargets(cmd *cobra.Command, args []string) error {
	if err := git.Available(); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	cfg, err = config.Load(path)
	if err != nil {
		return err
	}
	ctxs, err = context.MakeAll(cfg)
	if err != nil {
		return err
	}

	targets, err = selectTargets()
	return err
}

func selectTargets() ([]string, error) {
	home, err := context.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}

	if nameFlag != "" {
		path := filepath.Join(home, nameFlag)
		if _, ok := ctxs[path]; !ok {
			return nil, fmt.Errorf("repo '%s' not found in config", nameFlag)
		}
		return []string{path}, nil
	}
	if allFlag {
		return slices.Sorted(maps.Keys(ctxs)), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	for _, path := range slices.Sorted(maps.Keys(ctxs)) {
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{path}, nil
		}
	}
	return nil, fmt.Errorf("not a managed repository (cwd: %s)", cwd)
}

func errorf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", v...)
}

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
