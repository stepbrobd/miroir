package forge

import (
	"context"
	"errors"
	"net/http"

	gh "github.com/google/go-github/v84/github"
	"ysun.co/miroir/internal/config"
)

type ghForge struct {
	c *gh.Client
}

func newGithub(token string) *ghForge {
	c := gh.NewClient(nil).WithAuthToken(token)
	return &ghForge{c: c}
}

func ghPrivate(v config.Visibility) bool { return v == config.Private }

func (g *ghForge) Create(ctx context.Context, _ string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	repo := &gh.Repository{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		AutoInit:    gh.Ptr(false),
	}
	_, resp, err := g.c.Repositories.Create(ctx, "", repo)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusConflict) {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *ghForge) Update(ctx context.Context, user string, m Meta) error {
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

func (g *ghForge) Archive(ctx context.Context, user, name string, flag bool) error {
	repo := &gh.Repository{Archived: &flag}
	_, _, err := g.c.Repositories.Edit(ctx, user, name, repo)
	return err
}

func (g *ghForge) Delete(ctx context.Context, user, name string) error {
	_, err := g.c.Repositories.Delete(ctx, user, name)
	return err
}

func (g *ghForge) List(ctx context.Context, _ string) ([]string, error) {
	opt := &gh.RepositoryListByAuthenticatedUserOptions{
		Type:        "owner",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	repos, _, err := g.c.Repositories.ListByAuthenticatedUser(ctx, opt)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.GetName())
	}
	return names, nil
}

func ghIsArchived(err error) bool {
	var e *gh.ErrorResponse
	return errors.As(err, &e) && e.Response != nil &&
		e.Response.StatusCode == http.StatusForbidden
}

func (g *ghForge) Sync(ctx context.Context, user string, m Meta) error {
	err := g.Create(ctx, user, m)
	if err == nil {
		return nil
	}
	if err != ErrExists {
		return err
	}
	if err := g.Update(ctx, user, m); err != nil {
		if !ghIsArchived(err) {
			return err
		}
		// repo is archived on remote; unarchive, update, re-archive if needed
		if err := g.Archive(ctx, user, m.Name, false); err != nil {
			return err
		}
		return g.Update(ctx, user, m)
	}
	return nil
}
