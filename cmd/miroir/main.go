package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ysun.co/miroir/internal/config"
	"ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/display"
	"ysun.co/miroir/internal/forge"
	"ysun.co/miroir/internal/git"
)

var version = "dev"

type args struct {
	config string
	name   string
	all    bool
	force  bool
	rest   []string
}

func parseFlags(fs *flag.FlagSet, argv []string) args {
	var a args
	fs.StringVar(&a.config, "c", os.Getenv("MIROIR_CONFIG"), "config file path")
	fs.StringVar(&a.config, "config", os.Getenv("MIROIR_CONFIG"), "config file path")
	fs.StringVar(&a.name, "n", "", "target repo by name")
	fs.StringVar(&a.name, "name", "", "target repo by name")
	fs.BoolVar(&a.all, "a", false, "target all repos")
	fs.BoolVar(&a.all, "all", false, "target all repos")
	fs.BoolVar(&a.force, "f", false, "force operation")
	fs.BoolVar(&a.force, "force", false, "force operation")
	_ = fs.Parse(argv)
	a.rest = fs.Args()
	return a
}

func fatal(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", v...)
	os.Exit(1)
}

func errorf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", v...)
}

func getTargets(a args) ([]string, map[string]*context.Context, *config.Config) {
	if err := git.Available(); err != nil {
		fatal("%s", err)
	}
	if a.config == "" {
		fatal("no config file specified (use -c/--config or set MIROIR_CONFIG)")
	}
	cfg, err := config.Load(a.config)
	if err != nil {
		fatal("%s", err)
	}
	ctxs := context.MakeAll(cfg)
	home := context.ExpandHome(cfg.General.Home)

	if a.name != "" {
		path := filepath.Join(home, a.name)
		if _, ok := ctxs[path]; !ok {
			fatal("repo '%s' not found in config", a.name)
		}
		return []string{path}, ctxs, cfg
	}
	if a.all {
		return sortedKeys(ctxs), ctxs, cfg
	}

	cwd, err := os.Getwd()
	if err != nil {
		fatal("getwd: %s", err)
	}
	for _, path := range sortedKeys(ctxs) {
		if path == cwd || strings.HasPrefix(cwd, path+string(filepath.Separator)) {
			return []string{path}, ctxs, cfg
		}
	}
	fatal("not a managed repository (cwd: %s)", cwd)
	return nil, nil, nil
}

func sortedKeys(m map[string]*context.Context) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// runOn executes op on each target with display and concurrency
func runOn(targets []string, ctxs map[string]*context.Context, cfg *config.Config, op git.Op, force bool, extra []string) {
	nplatforms := len(cfg.Platform)
	nr := op.Remotes(nplatforms)

	var errors []struct{ repo, msg string }
	var errMu sync.Mutex
	addErr := func(repo, msg string) {
		errMu.Lock()
		errors = append(errors, struct{ repo, msg string }{repo, msg})
		errMu.Unlock()
	}

	if nr == 0 {
		// no display needed, run sequentially
		disp := display.New(1, 0, display.DefaultTheme)
		sem := make(chan struct{}, 1)
		for _, target := range targets {
			ctx := ctxs[target]
			err := op.Run(git.Params{
				Path: target, Ctx: ctx, Disp: disp,
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

		disp := display.New(rc, nr, display.DefaultTheme)
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

				ctx := ctxs[target]
				err := op.Run(git.Params{
					Path: target, Ctx: ctx, Disp: disp,
					Slot: slot, Sem: sem, Force: force, Args: extra,
				})
				if err != nil {
					name := filepath.Base(target)
					addErr(name, err.Error())
					disp.Repo(slot, fmt.Sprintf("error: %s", err))
				}
			}(target)
		}
		wg.Wait()
		disp.Finish()
	}

	if len(errors) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, e := range errors {
			errorf("%s :: %s", e.repo, e.msg)
		}
	}
}

func syncRepo(cfg *config.Config, disp *display.Display, slot int, sem chan struct{}, name string) error {
	repo, ok := cfg.Repo[name]
	if !ok {
		disp.Repo(slot, fmt.Sprintf("%s :: sync :: no repo config", name))
		repo = config.Repo{Visibility: config.Private}
	}
	disp.Repo(slot, fmt.Sprintf("%s :: sync", name))

	pnames := sortedPlatformKeys(cfg.Platform)
	var (
		mu     sync.Mutex
		errors []string
		wg     sync.WaitGroup
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
			impl, err := forge.Dispatch(*f, *t)
			if err != nil {
				disp.Remote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.Output(slot, j, err.Error())
				mu.Lock()
				errors = append(errors, fmt.Sprintf("%s/%s", pname, err))
				mu.Unlock()
				return
			}
			meta := forge.Meta{
				Name:     name,
				Desc:     repo.Description,
				Vis:      repo.Visibility,
				Archived: repo.Archived,
			}
			if err := impl.Sync(p.User, meta); err != nil {
				disp.Remote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.Output(slot, j, err.Error())
				mu.Lock()
				errors = append(errors, fmt.Sprintf("%s/%s", pname, err))
				mu.Unlock()
			} else {
				disp.Remote(slot, j, fmt.Sprintf("%s :: done", pname))
				disp.Output(slot, j, fmt.Sprintf("synced on %s", f))
			}
		}(j, pname, cfg.Platform[pname])
	}
	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}
	return nil
}

func sortedPlatformKeys(m map[string]config.Platform) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func runSync(a args) {
	targets, _, cfg := getTargets(a)
	nrepos := len(targets)
	nremotes := len(cfg.Platform)
	rc := min(cfg.General.Concurrency.Repo, nrepos)
	rcRemote := cfg.General.Concurrency.Remote
	mc := nremotes
	if rcRemote > 0 {
		mc = min(rcRemote, nremotes)
	}

	disp := display.New(rc, nremotes, display.DefaultTheme)
	pool := make(chan int, rc)
	for i := range rc {
		pool <- i
	}
	sem := make(chan struct{}, mc)

	var (
		errors []struct{ repo, msg string }
		errMu  sync.Mutex
		wg     sync.WaitGroup
	)

	for _, target := range targets {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			slot := <-pool
			defer func() { pool <- slot }()
			disp.Clear(slot)

			name := filepath.Base(target)
			if err := syncRepo(cfg, disp, slot, sem, name); err != nil {
				errMu.Lock()
				errors = append(errors, struct{ repo, msg string }{name, err.Error()})
				errMu.Unlock()
			}
		}(target)
	}
	wg.Wait()
	disp.Finish()

	if len(errors) > 0 {
		fmt.Fprintln(os.Stderr)
		for _, e := range errors {
			errorf("%s :: %s", e.repo, e.msg)
		}
	}
}

const usage = `miroir - repo manager wannabe

Usage: miroir <command> [flags] [args...]

Commands:
  init   initialize repo(s) (destructive, uncommitted changes will be lost)
  fetch  fetch from all remotes
  pull   pull from origin
  push   push to all remotes
  exec   execute command in repo(s)
  sync   sync metadata to all forges

Flags:
  -c, --config  config file path (or set MIROIR_CONFIG)
  -n, --name    target specific repo by name
  -a, --all     target all repos
  -f, --force   force operation
  -v, --version print version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "-v" || cmd == "--version" || cmd == "version" {
		fmt.Println(version)
		return
	}
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Print(usage)
		return
	}

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	a := parseFlags(fs, os.Args[2:])

	var op git.Op
	switch cmd {
	case "init":
		op = git.Init{}
	case "fetch":
		op = git.Fetch{}
	case "pull":
		op = git.Pull{}
	case "push":
		op = git.Push{}
	case "exec":
		op = git.Exec{}
	case "sync":
		runSync(a)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	targets, ctxs, cfg := getTargets(a)
	runOn(targets, ctxs, cfg, op, a.force, a.rest)
}
