package forge

import (
	"fmt"
	"net/http"

	"code.gitea.io/sdk/gitea"
	"ysun.co/miroir/internal/config"
)

type cbForge struct {
	c *gitea.Client
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

func (g *cbForge) Create(_ string, m Meta) error {
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

func (g *cbForge) Update(user string, m Meta) error {
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

func (g *cbForge) Archive(user, name string, flag bool) error {
	_, _, err := g.c.EditRepo(user, name, gitea.EditRepoOption{
		Archived: &flag,
	})
	return err
}

func (g *cbForge) Delete(user, name string) error {
	_, err := g.c.DeleteRepo(user, name)
	return err
}

func (g *cbForge) List(_ string) ([]string, error) {
	repos, _, err := g.c.ListMyRepos(gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{PageSize: 50},
	})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names, nil
}

func (g *cbForge) Sync(user string, m Meta) error {
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
