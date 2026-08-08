package index

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/sourcegraph/zoekt/search"
	"github.com/sourcegraph/zoekt/web"

	"ysun.co/miroir/config"
	"ysun.co/miroir/workspace"
)

// cfg holds resolved daemon configuration
type Cfg struct {
	Listen   string
	Database string // absolute path to shard dir
	Interval time.Duration
	Bare     bool
	Include  []string
	Env      CmdEnv

	// managed repos derived from miroir config
	Home  string
	Repos []Repo
}

// indexRepo is a seam swapped out by tests
var indexRepo = IndexRepo

// cfgFrom builds a daemon config from a validated miroir config
func CfgFrom(c *config.Config) (*Cfg, error) {
	home, err := workspace.ExpandHome(c.General.Home)
	if err != nil {
		return nil, err
	}
	db, err := workspace.ExpandHome(c.Index.Database)
	if err != nil {
		return nil, fmt.Errorf("expand database path: %w", err)
	}
	include := make([]string, 0, len(c.Index.Include))
	for _, inc := range c.Index.Include {
		p, err := workspace.ExpandHome(inc)
		if err != nil {
			return nil, fmt.Errorf("expand include path: %w", err)
		}
		include = append(include, p)
	}

	// config validation guarantees exactly one origin platform
	var origin config.Platform
	for _, p := range c.Platform {
		if p.Origin {
			origin = p
			break
		}
	}

	var repos []Repo
	for _, name := range slices.Sorted(maps.Keys(c.Repo)) {
		repo := c.Repo[name]
		if repo.Archived {
			continue
		}
		branch := c.General.Branch
		if repo.Branch != nil {
			branch = *repo.Branch
		}
		webURL, webURLType := repoWebMetadata(origin, name)
		repos = append(repos, Repo{
			Name:       name,
			IndexName:  repoIndexName(origin, name),
			URI:        workspace.MakeURI(origin.Access, origin.Domain, origin.User, name),
			Branch:     branch,
			WebURL:     webURL,
			WebURLType: webURLType,
		})
	}

	return &Cfg{
		Listen:   c.Index.Listen,
		Database: filepath.Clean(db),
		Interval: time.Duration(c.Index.Interval) * time.Second,
		Bare:     c.Index.Bare,
		Include:  include,
		Env:      CmdEnv(workspace.MergeEnv(c.General.Env)),
		Home:     filepath.Clean(home),
		Repos:    repos,
	}, nil
}

func repoWebMetadata(p config.Platform, repo string) (string, string) {
	forge := config.ResolveForge(p)
	if forge == nil {
		return "", ""
	}

	var webURLType string
	switch *forge {
	case config.Github:
		webURLType = "github"
	case config.Gitlab:
		webURLType = "gitlab"
	case config.Codeberg:
		webURLType = "gitea"
	case config.Sourcehut:
		webURLType = "cgit"
	default:
		return "", ""
	}

	return fmt.Sprintf("https://%s/%s", p.Domain, path.Join(p.User, repo)), webURLType
}

func repoIndexName(p config.Platform, repo string) string {
	return path.Join(p.Domain, p.User, repo)
}

// run starts the daemon and blocks until ctx is cancelled
func Run(ctx context.Context, c *Cfg) error {
	if err := os.MkdirAll(c.Database, 0o755); err != nil {
		return fmt.Errorf("create database dir: %w", err)
	}

	// searcher loads shards in background and hot-reloads via directory watcher
	searcher, err := search.NewDirectorySearcherFast(c.Database)
	if err != nil {
		return fmt.Errorf("searcher: %w", err)
	}
	defer searcher.Close()

	// clone the top template set so we don't mutate the package-level global
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

	// cycles get their own cancel so a server failure can abort them
	cycleCtx, cancelCycles := context.WithCancel(ctx)
	defer cancelCycles()

	var cycleWg sync.WaitGroup
	var cycleMu sync.Mutex
	startCycle := func() {
		cycleWg.Go(func() {
			if !cycleMu.TryLock() {
				log.Info("cycle skipped, previous still running")
				return
			}
			defer cycleMu.Unlock()
			cycle(cycleCtx, c)
		})
	}

	// run initial cycle in background so the server is available immediately
	startCycle()

	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutting down")
			shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := httpSrv.Shutdown(shut)
			cycleWg.Wait()
			if err != nil {
				return err
			}
			return ctx.Err()
		case err := <-errCh:
			// abort the running cycle and release server resources
			cancelCycles()
			httpSrv.Close()
			cycleWg.Wait()
			return err
		case <-ticker.C:
			startCycle()
		}
	}
}

// cycle runs one fetch+index pass
func cycle(ctx context.Context, c *Cfg) {
	log.Info("cycle start")
	start := time.Now()
	var n int
	var discovered []string
	includeReady := len(c.Include) == 0

	if err := ctx.Err(); err != nil {
		return
	}
	if err := cleanupManagedRepoDirs(c); err != nil {
		log.Error("cleanup repos failed", "err", err)
	}

	// fetch and index each managed repo immediately
	for _, r := range c.Repos {
		if err := ctx.Err(); err != nil {
			return
		}
		p, err := Fetch(ctx, c.Home, r, c.Bare, c.Env)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("fetch failed", "repo", r.Name, "err", err)
			continue
		}
		if err := indexRepo(p, c.Database, r.IndexName); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error("index failed", "repo", r.Name, "err", err)
			continue
		}
		if err := ctx.Err(); err != nil {
			return
		}
		if err := cleanupManagedShardsForRepo(c.Database, p, r.IndexName); err != nil {
			log.Error("cleanup managed shards failed", "repo", r.Name, "err", err)
		}
		n++
	}

	// discover include repos (no fetch, index only)
	if len(c.Include) > 0 {
		if err := ctx.Err(); err != nil {
			return
		}
		var err error
		discovered, err = Discover(c.Include)
		if err != nil {
			log.Error("discover failed", "err", err)
		} else {
			includeReady = true
			for _, p := range discovered {
				if err := ctx.Err(); err != nil {
					return
				}
				if err := indexRepo(p, c.Database, ""); err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Error("index failed", "repo", p, "err", err)
					continue
				}
				n++
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return
	}
	if err := cleanupShards(c, discovered, includeReady); err != nil {
		log.Error("cleanup shards failed", "err", err)
	}

	log.Info("cycle done", "repos", n, "elapsed", time.Since(start).Round(time.Millisecond))
}
