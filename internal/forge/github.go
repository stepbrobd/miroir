package forge

import (
	"context"
	"net/http"

	gh "github.com/google/go-github/v69/github"
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

func (g *ghForge) Create(_ string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	repo := &gh.Repository{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		AutoInit:    gh.Ptr(false),
	}
	_, resp, err := g.c.Repositories.Create(context.Background(), "", repo)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusConflict) {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *ghForge) Update(user string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := ghPrivate(m.Vis)
	repo := &gh.Repository{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		Archived:    &m.Archived,
	}
	_, _, err := g.c.Repositories.Edit(context.Background(), user, m.Name, repo)
	return err
}

func (g *ghForge) Archive(user, name string, flag bool) error {
	repo := &gh.Repository{Archived: &flag}
	_, _, err := g.c.Repositories.Edit(context.Background(), user, name, repo)
	return err
}

func (g *ghForge) Delete(user, name string) error {
	_, err := g.c.Repositories.Delete(context.Background(), user, name)
	return err
}

func (g *ghForge) List(_ string) ([]string, error) {
	opt := &gh.RepositoryListByAuthenticatedUserOptions{
		Type:        "owner",
		ListOptions: gh.ListOptions{PerPage: 100},
	}
	repos, _, err := g.c.Repositories.ListByAuthenticatedUser(context.Background(), opt)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.GetName())
	}
	return names, nil
}

func (g *ghForge) Sync(user string, m Meta) error {
	err := g.Create(user, m)
	if err == nil {
		return nil
	}
	if err != ErrExists {
		return err
	}
	if err := g.Update(user, m); err != nil {
		return err
	}
	if m.Archived {
		return g.Archive(user, m.Name, true)
	}
	return nil
}
