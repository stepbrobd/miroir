package miroir

import (
	"context"
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

func syncRepo(ctx context.Context, cfg *config.Config, disp gitops.Reporter, slot int, sem chan struct{}, name string) []remoteErr {
	repo := cfg.Repo[name]
	disp.Repo(slot, fmt.Sprintf("%s :: sync", name))

	pnames := slices.Sorted(maps.Keys(cfg.Platform))
	var (
		mu   sync.Mutex
		errs []remoteErr
		wg   sync.WaitGroup
	)

	for j, pname := range pnames {
		wg.Add(1)
		go func(j int, pname string, p config.Platform) {
			defer wg.Done()
			disp.Remote(slot, j, fmt.Sprintf("%s :: waiting...", pname))
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

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
			impl, err := forge.Dispatch(*f, *t, p.Domain)
			if err != nil {
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, remoteErr{pname, err.Error()})
				mu.Unlock()
				return
			}
			runCtx, cancel := context.WithTimeout(ctx, syncTimeout)
			defer cancel()
			meta := forge.Meta{
				Name:     name,
				Desc:     repo.Description,
				Vis:      repo.Visibility,
				Archived: repo.Archived,
			}
			if err := impl.Sync(runCtx, p.User, meta); err != nil {
				if ctx.Err() != nil {
					return
				}
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, remoteErr{pname, err.Error()})
				mu.Unlock()
			} else {
				disp.Remote(slot, j, fmt.Sprintf("%s :: done", pname))
				disp.Output(slot, j, fmt.Sprintf("synced on %s", f))
			}
		}(j, pname, cfg.Platform[pname])
	}
	wg.Wait()

	return errs
}

// RunSync syncs repo metadata to all configured forges for the given names
// ctx must be non-nil
func RunSync(ctx context.Context, cfg *config.Config, names []string, disp gitops.Reporter) error {
	var (
		errs  []repoRemoteErr
		errMu sync.Mutex
	)
	rc := min(cfg.General.Concurrency.Repo, len(names))
	mc := remoteSlots(cfg.General.Concurrency.Remote, len(cfg.Platform))
	pooled(ctx, names, rc, mc, disp, func(slot int, sem chan struct{}, name string) {
		for _, re := range syncRepo(ctx, cfg, disp, slot, sem, name) {
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
