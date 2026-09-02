package forge

import (
	"context"
	"fmt"
	"net/http"

	"code.gitea.io/sdk/gitea"
	"ysun.co/miroir/config"
)

// the gitea sdk binds the context to the client, so every call builds a
// client for its own ctx over one shared transport
type cbForge struct {
	url   string
	token string
	hc    *http.Client
}

func newCodeberg(token, domain string) (*cbForge, error) {
	g := &cbForge{url: "https://" + domain, token: token, hc: &http.Client{}}
	if _, err := g.client(context.Background()); err != nil {
		return nil, fmt.Errorf("codeberg client: %w", err)
	}
	return g, nil
}

func (g *cbForge) client(ctx context.Context) (*gitea.Client, error) {
	return gitea.NewClient(g.url,
		gitea.SetHTTPClient(g.hc),
		gitea.SetToken(g.token),
		gitea.SetGiteaVersion(""),
		gitea.SetContext(ctx),
	)
}

func cbPrivate(v config.Visibility) bool { return v == config.Private }

func cbCreate(c *gitea.Client, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := cbPrivate(m.Vis)
	_, resp, err := c.CreateRepo(gitea.CreateRepoOption{
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

func cbUpdate(c *gitea.Client, user string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	priv := cbPrivate(m.Vis)
	_, _, err := c.EditRepo(user, m.Name, gitea.EditRepoOption{
		Name:        &m.Name,
		Description: &desc,
		Private:     &priv,
		Archived:    &m.Archived,
	})
	return err
}

func cbArchive(c *gitea.Client, user, name string, flag bool) error {
	_, _, err := c.EditRepo(user, name, gitea.EditRepoOption{Archived: &flag})
	return err
}

func (g *cbForge) Sync(ctx context.Context, user string, m Meta) error {
	c, err := g.client(ctx)
	if err != nil {
		return err
	}
	repo, resp, err := c.GetRepo(user, m.Name)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			if err := cbCreate(c, m); err != nil {
				return err
			}
			if m.Archived {
				return cbArchive(c, user, m.Name, true)
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
		if err := cbArchive(c, user, m.Name, false); err != nil {
			return err
		}
	}
	return cbUpdate(c, user, m)
}
