package main

import (
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/git"
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

	// set by resolveTargets; must be populated before subcommand RunE
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

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
