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

func newCodeberg(token string) (*cbForge, error) {
	c, err := gitea.NewClient("https://codeberg.org",
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

func (g *cbForge) Create(ctx context.Context, _ string, m Meta) error {
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

func (g *cbForge) Update(ctx context.Context, user string, m Meta) error {
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

func (g *cbForge) Archive(ctx context.Context, user, name string, flag bool) error {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	_, _, err := g.c.EditRepo(user, name, gitea.EditRepoOption{
		Archived: &flag,
	})
	return err
}

func (g *cbForge) Delete(ctx context.Context, user, name string) error {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	_, err := g.c.DeleteRepo(user, name)
	return err
}

func (g *cbForge) List(ctx context.Context, _ string) ([]string, error) {
	g.withCtx(ctx)
	defer g.mu.Unlock()
	opt := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{PageSize: 50, Page: 1},
	}
	var names []string
	for {
		repos, _, err := g.c.ListMyRepos(opt)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			names = append(names, r.Name)
		}
		if len(repos) < opt.PageSize {
			break
		}
		opt.Page++
	}
	return names, nil
}

func (g *cbForge) Sync(ctx context.Context, user string, m Meta) error {
	err := g.Create(ctx, user, m)
	if err == nil {
		return nil
	}
	if err != ErrExists {
		return err
	}
	return g.Update(ctx, user, m)
}
