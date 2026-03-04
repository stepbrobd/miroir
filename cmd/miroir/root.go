package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/git"
)

var (
	cfgFlag   string
	nameFlag  string
	allFlag   bool
	forceFlag bool

	// populated by resolveTargets before any subcommand runs
	targets []string
	ctxs    map[string]*context.Context
	cfg     *config.Config
)

var root = &cobra.Command{
	Use:           "miroir",
	Short:         "Repo manager wannabe?",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	f := root.PersistentFlags()
	f.StringVarP(&cfgFlag, "config", "c", "", "config file path (or set MIROIR_CONFIG)")
	f.StringVarP(&nameFlag, "name", "n", "", "target repo by name")
	f.BoolVarP(&allFlag, "all", "a", false, "target all repos")
	f.BoolVarP(&forceFlag, "force", "f", false, "force operation")
}

// priority: flag > MIROIR_CONFIG env > XDG via viper
func configPath() (string, error) {
	if cfgFlag != "" {
		return cfgFlag, nil
	}
	if env := os.Getenv("MIROIR_CONFIG"); env != "" {
		return env, nil
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		v.AddConfigPath(filepath.Join(xdg, "miroir"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	v.AddConfigPath(filepath.Join(home, ".config", "miroir"))

	if err := v.ReadInConfig(); err != nil {
		return "", fmt.Errorf("no config file found (use -c/--config or set MIROIR_CONFIG)")
	}
	return v.ConfigFileUsed(), nil
}

// used as PersistentPreRunE for subcommands that need targets
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
	ctxs = context.MakeAll(cfg)

	targets, err = selectTargets()
	return err
}

func selectTargets() ([]string, error) {
	home := context.ExpandHome(cfg.General.Home)

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
