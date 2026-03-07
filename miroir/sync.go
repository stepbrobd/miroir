// package miroir contains high-level orchestration for miroir workflows
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
	return fmt.Errorf("%d repo(s) failed to sync", len(errs))
}

func syncRepo(cfg *config.Config, disp gitops.Reporter, slot int, sem chan struct{}, name string) []remoteErr {
	repo, ok := cfg.Repo[name]
	if !ok {
		disp.Repo(slot, fmt.Sprintf("%s :: sync :: no repo config", name))
		repo = config.Repo{Visibility: config.Private}
	}
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
			impl, err := forge.Dispatch(*f, *t, p.Domain)
			if err != nil {
				disp.ErrorRemote(slot, j, fmt.Sprintf("%s :: error", pname))
				disp.ErrorOutput(slot, j, err.Error())
				mu.Lock()
				errs = append(errs, remoteErr{pname, err.Error()})
				mu.Unlock()
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
			defer cancel()
			meta := forge.Meta{
				Name:     name,
				Desc:     repo.Description,
				Vis:      repo.Visibility,
				Archived: repo.Archived,
			}
			if err := impl.Sync(ctx, p.User, meta); err != nil {
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

// runSync syncs repo metadata to all configured forges for the given names
func RunSync(cfg *config.Config, names []string, disp gitops.Reporter) error {
	nrepos := len(names)
	nremotes := len(cfg.Platform)
	rc := min(cfg.General.Concurrency.Repo, nrepos)
	rcRemote := cfg.General.Concurrency.Remote
	mc := nremotes
	if rcRemote > 0 {
		mc = min(rcRemote, nremotes)
	}

	pool := make(chan int, rc)
	for i := range rc {
		pool <- i
	}
	sem := make(chan struct{}, mc)

	var (
		err   []repoRemoteErr
		errMu sync.Mutex
		wg    sync.WaitGroup
	)

	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			slot := <-pool
			defer func() { pool <- slot }()
			disp.Clear(slot)

			for _, re := range syncRepo(cfg, disp, slot, sem, name) {
				errMu.Lock()
				err = append(err, repoRemoteErr{name, re.remote, re.msg})
				errMu.Unlock()
			}
		}(name)
	}
	wg.Wait()
	disp.Finish()

	if len(err) > 0 {
		return reportRepoRemoteErrors(err)
	}
	return nil
}
