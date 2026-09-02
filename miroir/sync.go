package miroir

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"ysun.co/miroir/config"
	"ysun.co/miroir/forge"
	"ysun.co/miroir/gitops"
)

const syncTimeout = 30 * time.Second

type repoRemoteErr struct {
	repo   string
	remote string
	msg    string
}

type remoteErr struct {
	remote string
	msg    string
}

func reportRepoRemoteErrors(errs []repoRemoteErr) error {
	for _, err := range errs {
		log.Error("sync failed", "repo", err.repo, "remote", err.remote, "error", err.msg)
	}
	return fmt.Errorf("%d sync failure(s)", len(errs))
}

// platform is one configured forge resolved once per sync run
// forge is nil when skip names the reason
type platform struct {
	name  string
	user  string
	kind  config.Forge
	forge forge.Forge
	skip  string
}

// resolvePlatforms builds one forge client per platform in name order
func resolvePlatforms(cfg *config.Config) ([]platform, error) {
	names := slices.Sorted(maps.Keys(cfg.Platform))
	platforms := make([]platform, 0, len(names))
	for _, name := range names {
		p := cfg.Platform[name]
		entry := platform{name: name, user: p.User}
		kind := config.ResolveForge(p)
		token := config.ResolveToken(name, p)
		switch {
		case kind == nil:
			entry.skip = "unknown forge"
		case token == nil:
			entry.skip = "no token"
		default:
			impl, err := forge.Dispatch(*kind, *token, p.Domain)
			if err != nil {
				return nil, fmt.Errorf("platform %s: %w", name, err)
			}
			entry.kind = *kind
			entry.forge = impl
		}
		platforms = append(platforms, entry)
	}
	return platforms, nil
}

// syncMeta gives the forge a second pass when the repo appeared between
// its read and its create, that pass reads it back as an update
func syncMeta(ctx context.Context, p platform, meta forge.Meta) error {
	runCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	err := p.forge.Sync(runCtx, p.user, meta)
	if errors.Is(err, forge.ErrExists) {
		return p.forge.Sync(runCtx, p.user, meta)
	}
	return err
}

func syncRepo(ctx context.Context, cfg *config.Config, platforms []platform, disp gitops.Reporter, slot int, sem chan struct{}, name string) []remoteErr {
	repo := cfg.Repo[name]
	disp.Repo(slot, fmt.Sprintf("%s :: sync", name))
	meta := forge.Meta{
		Name:     name,
		Desc:     repo.Description,
		Vis:      repo.Visibility,
		Archived: repo.Archived,
	}

	var (
		mu   sync.Mutex
		errs []remoteErr
		wg   sync.WaitGroup
	)
	for j, p := range platforms {
		wg.Go(func() {
			if p.forge == nil {
				disp.Remote(slot, j, fmt.Sprintf("%s :: skipped", p.name))
				disp.Output(slot, j, p.skip)
				return
			}
			disp.Remote(slot, j, fmt.Sprintf("%s :: waiting...", p.name))
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			disp.Remote(slot, j, fmt.Sprintf("%s :: syncing...", p.name))
			if err := syncMeta(ctx, p, meta); err != nil {
				if ctx.Err() != nil {
					return
				}
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", p.name))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, remoteErr{p.name, err.Error()})
				mu.Unlock()
				return
			}
			disp.Remote(slot, j, fmt.Sprintf("%s :: done", p.name))
			disp.Output(slot, j, fmt.Sprintf("synced on %s", p.kind))
		})
	}
	wg.Wait()
	return errs
}

// RunSync syncs repo metadata to all configured forges for the given names
// ctx must be non-nil
func RunSync(ctx context.Context, cfg *config.Config, names []string, disp gitops.Reporter) error {
	platforms, err := resolvePlatforms(cfg)
	if err != nil {
		return err
	}

	var (
		errs  []repoRemoteErr
		errMu sync.Mutex
	)
	rc := min(cfg.General.Concurrency.Repo, len(names))
	mc := remoteSlots(cfg.General.Concurrency.Remote, len(cfg.Platform))
	pooled(ctx, names, rc, mc, disp, func(slot int, sem chan struct{}, name string) {
		for _, re := range syncRepo(ctx, cfg, platforms, disp, slot, sem, name) {
			errMu.Lock()
			errs = append(errs, repoRemoteErr{name, re.remote, re.msg})
			errMu.Unlock()
		}
	})

	if err := ctx.Err(); err != nil {
		return err
	}
	if len(errs) > 0 {
		return reportRepoRemoteErrors(errs)
	}
	return nil
}
