package forge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v84/github"
	"ysun.co/miroir/config"
)

type ghForge struct {
	c *gh.Client
}

func newGithub(token, domain string) (*ghForge, error) {
	c := gh.NewClient(nil)
	if !strings.EqualFold(domain, "github.com") {
		var err error
		// go-github appends /api/v3/ and /api/uploads/ itself
		c, err = c.WithEnterpriseURLs("https://"+domain, "https://"+domain)
		if err != nil {
			return nil, fmt.Errorf("github client: %w", err)
		}
	}
	return &ghForge{c: c.WithAuthToken(token)}, nil
}

func ghPrivate(v config.Visibility) bool { return v == config.Private }

func (g *ghForge) create(ctx context.Context, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	repo := &gh.Repository{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		AutoInit:    new(false),
	}
	_, resp, err := g.c.Repositories.Create(ctx, "", repo)
	if err != nil {
		if resp != nil &&
			(resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusConflict) &&
			strings.Contains(err.Error(), "name already exists") {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *ghForge) update(ctx context.Context, user string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	repo := &gh.Repository{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		Archived:    &m.Archived,
	}
	_, _, err := g.c.Repositories.Edit(ctx, user, m.Name, repo)
	return err
}

func (g *ghForge) archive(ctx context.Context, user, name string, flag bool) error {
	repo := &gh.Repository{Archived: &flag}
	_, _, err := g.c.Repositories.Edit(ctx, user, name, repo)
	return err
}

func (g *ghForge) Sync(ctx context.Context, user string, m Meta) error {
	repo, resp, err := g.c.Repositories.Get(ctx, user, m.Name)
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
	// update only when live state differs from config
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	if repo.GetDescription() == desc && repo.GetPrivate() == priv && repo.GetArchived() == m.Archived {
		return nil
	}
	// archived repos are read-only on github, unarchive before editing
	if repo.GetArchived() {
		if err := g.archive(ctx, user, m.Name, false); err != nil {
			return err
		}
	}
	return g.update(ctx, user, m)
}
