package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
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
	ttyFlag   bool
	noTTYFlag bool

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
	f.BoolVar(&ttyFlag, "tty", false, "force TTY output")
	f.BoolVar(&noTTYFlag, "no-tty", false, "force plain output")
	root.MarkFlagsMutuallyExclusive("tty", "no-tty")
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

func loadConfig(cmd *cobra.Command, args []string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	cfg, err = config.Load(path)
	return err
}

func resolveTargets(cmd *cobra.Command, args []string) error {
	if err := git.Available(); err != nil {
		return err
	}
	if err := loadConfig(cmd, args); err != nil {
		return err
	}
	var err error
	ctxs, err = context.MakeAll(cfg)
	if err != nil {
		return err
	}
	targets, err = selectTargets()
	return err
}

func flatNames(paths []string, home string) ([]string, error) {
	names := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return nil, fmt.Errorf("repo path %q: %w", path, err)
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is outside workspace %q", path, home)
		}
		if strings.Contains(rel, string(filepath.Separator)) {
			return nil, fmt.Errorf("repo path %q is not flat under workspace %q", path, home)
		}
		if _, ok := seen[rel]; ok {
			return nil, fmt.Errorf("duplicate repo name %q under workspace %q", rel, home)
		}
		seen[rel] = struct{}{}
		names = append(names, rel)
	}
	slices.Sort(names)
	return names, nil
}

// resolveNames picks repo names from sorted candidates via flags or cwd
func resolveNames(names []string, home string) ([]string, error) {
	if nameFlag != "" {
		if !slices.Contains(names, nameFlag) {
			return nil, fmt.Errorf("repo '%s' not found in config", nameFlag)
		}
		return []string{nameFlag}, nil
	}
	if allFlag {
		return names, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	for _, n := range names {
		path := filepath.Join(home, n)
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{n}, nil
		}
	}
	return nil, fmt.Errorf("not a managed repository (cwd: %s)", cwd)
}

func selectTargets() ([]string, error) {
	home, err := context.ExpandHome(cfg.General.Home)
	if err != nil {
		return nil, err
	}
	names, err := flatNames(slices.Collect(maps.Keys(ctxs)), home)
	if err != nil {
		return nil, err
	}
	matched, err := resolveNames(names, home)
	if err != nil {
		return nil, err
	}
	paths := make([]string, len(matched))
	for i, n := range matched {
		paths[i] = filepath.Join(home, n)
	}
	return paths, nil
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

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
