package index

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sourcegraph/zoekt/search"
	"github.com/sourcegraph/zoekt/web"

	"ysun.co/miroir/internal/config"
	mirctx "ysun.co/miroir/internal/context"
)

// Cfg holds resolved daemon configuration
type Cfg struct {
	Listen   string
	Database string // absolute path to shard dir
	Interval time.Duration
	Bare     bool
	Include  []string

	// managed repos derived from miroir config
	Home  string
	Repos []Repo
}

// CfgFrom builds a Cfg from miroir config
// also sets process-level environment variables from general.env
// so that all child processes (git clone/fetch) inherit them
func CfgFrom(c *config.Config) (*Cfg, error) {
	for k, v := range c.General.Env {
		if err := os.Setenv(k, v); err != nil {
			return nil, fmt.Errorf("setenv %s: %w", k, err)
		}
	}
	home, err := mirctx.ExpandHome(c.General.Home)
	if err != nil {
		return nil, err
	}
	db, err := mirctx.ExpandHome(c.Index.Database)
	if err != nil {
		return nil, fmt.Errorf("expand database path: %w", err)
	}

	if c.Index.Interval <= 0 {
		return nil, fmt.Errorf("index.interval must be positive, got %d", c.Index.Interval)
	}

	var repos []Repo
	// deterministic iteration
	pnames := slices.Sorted(maps.Keys(c.Platform))
	for _, name := range slices.Sorted(maps.Keys(c.Repo)) {
		repo := c.Repo[name]
		if repo.Archived {
			continue
		}
		branch := c.General.Branch
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		for _, pn := range pnames {
			p := c.Platform[pn]
			if !p.Origin {
				continue
			}
			uri := mirctx.MakeURI(p.Access, p.Domain, p.User, name)
			repos = append(repos, Repo{Name: name, URI: uri, Branch: branch})
			break
		}
	}

	return &Cfg{
		Listen:   c.Index.Listen,
		Database: filepath.Clean(db),
		Interval: time.Duration(c.Index.Interval) * time.Second,
		Bare:     c.Index.Bare,
		Include:  c.Index.Include,
		Home:     filepath.Clean(home),
		Repos:    repos,
	}, nil
}

// Run starts the daemon, blocks until ctx is cancelled
func Run(ctx context.Context, c *Cfg) error {
	if err := os.MkdirAll(c.Database, 0o755); err != nil {
		return fmt.Errorf("create database dir: %w", err)
	}

	// searcher hot-reloads shards via directory watcher
	searcher, err := search.NewDirectorySearcher(c.Database)
	if err != nil {
		return fmt.Errorf("searcher: %w", err)
	}
	defer searcher.Close()

	// clone web.Top so we don't mutate the package-level global
	top, err := web.Top.Clone()
	if err != nil {
		return fmt.Errorf("clone templates: %w", err)
	}
	for k, v := range web.TemplateText {
		if _, err := top.New(k).Parse(v); err != nil {
			return fmt.Errorf("parse template %s: %w", k, err)
		}
	}

	srv := &web.Server{
		Searcher: searcher,
		Top:      top,
		HTML:     true,
		RPC:      true,
	}
	mux, err := web.NewMux(srv)
	if err != nil {
		return fmt.Errorf("web mux: %w", err)
	}
	httpSrv := &http.Server{Addr: c.Listen, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		log.Info("serving", "addr", c.Listen)
		if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var cycleMu sync.Mutex
	runCycle := func() {
		if !cycleMu.TryLock() {
			log.Info("cycle skipped, previous still running")
			return
		}
		defer cycleMu.Unlock()
		cycle(c)
	}

	// run initial cycle in background so the server is available immediately
	go runCycle()

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down")
			shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := httpSrv.Shutdown(shut)
			cancel()
			return err
		case err := <-errCh:
			return err
		case <-ticker.C:
			runCycle()
		}
	}
}

// cycle runs one fetch+index pass
func cycle(c *Cfg) {
	log.Info("cycle start")
	start := time.Now()
	var n int

	// fetch and index each managed repo immediately
	for _, r := range c.Repos {
		p, err := Fetch(c.Home, r, c.Bare)
		if err != nil {
			log.Error("fetch failed", "repo", r.Name, "err", err)
			continue
		}
		if err := IndexRepo(p, c.Database, nil); err != nil {
			log.Error("index failed", "repo", r.Name, "err", err)
			continue
		}
		n++
	}

	// discover include repos (no fetch, index only)
	if len(c.Include) > 0 {
		discovered, err := Discover(c.Include)
		if err != nil {
			log.Error("discover failed", "err", err)
		} else {
			for _, p := range discovered {
				if err := IndexRepo(p, c.Database, nil); err != nil {
					log.Error("index failed", "repo", p, "err", err)
					continue
				}
				n++
			}
		}
	}

	log.Info("cycle done", "repos", n, "elapsed", time.Since(start).Round(time.Millisecond))
}
