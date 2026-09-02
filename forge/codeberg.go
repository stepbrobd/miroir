package forge

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"code.gitea.io/sdk/gitea"
	"ysun.co/miroir/config"
)

type cbForge struct {
	mu sync.Mutex
	c  *gitea.Client
}

func newCodeberg(token, domain string) (*cbForge, error) {
	c, err := gitea.NewClient("https://"+domain,
		gitea.SetToken(token),
		gitea.SetGiteaVersion(""),
	)
	if err != nil {
		return nil, fmt.Errorf("codeberg client: %w", err)
	}
	return &cbForge{c: c}, nil
}

func cbPrivate(v config.Visibility) bool { return v == config.Private }

// gitea sdk uses client-level context so withCtx holds the mutex around SetContext
func (g *cbForge) withCtx(ctx context.Context) {
	g.mu.Lock()
	g.c.SetContext(ctx)
}

func (g *cbForge) create(ctx context.Context, m Meta) error {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	desc := descOrEmpty(m.Desc)
	priv := cbPrivate(m.Vis)
	_, resp, err := g.c.CreateRepo(gitea.CreateRepoOption{
		Name:        m.Name,
		Description: desc,
		Private:     priv,
		AutoInit:    false,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusConflict {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *cbForge) update(ctx context.Context, user string, m Meta) error {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	desc := descOrEmpty(m.Desc)
	priv := cbPrivate(m.Vis)
	_, _, err := g.c.EditRepo(user, m.Name, gitea.EditRepoOption{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		Archived:    &m.Archived,
	})
	return err
}

func (g *cbForge) archive(ctx context.Context, user, name string, flag bool) error {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	_, _, err := g.c.EditRepo(user, name, gitea.EditRepoOption{Archived: &flag})
	return err
}

func (g *cbForge) Sync(ctx context.Context, user string, m Meta) error {
	g.withCtx(ctx)
	repo, resp, err := g.c.GetRepo(user, m.Name)
	g.mu.Unlock()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			if err := g.create(ctx, m); err != nil {
				return err
			}
			if m.Archived {
				return g.archive(ctx, user, m.Name, true)
			}
			return nil
		}
		return err
	}
	desc := descOrEmpty(m.Desc)
	priv := cbPrivate(m.Vis)
	if repo.Description == desc && repo.Private == priv && repo.Archived == m.Archived {
		return nil
	}
	// archived repos are read-only on gitea, unarchive before editing
	if repo.Archived {
		if err := g.archive(ctx, user, m.Name, false); err != nil {
			return err
		}
	}
	return g.update(ctx, user, m)
}
