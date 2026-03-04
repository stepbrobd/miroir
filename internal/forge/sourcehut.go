package forge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"ysun.co/miroir/internal/config"
)

const srhtEndpoint = "https://git.sr.ht/query"

type srhtForge struct {
	c *graphql.Client
}

// srhtTransport injects bearer token into every request
type srhtTransport struct {
	token string
	base  http.RoundTripper
}

func (t *srhtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

func newSourcehut(token string) *srhtForge {
	hc := &http.Client{
		Transport: &srhtTransport{token: token, base: http.DefaultTransport},
	}
	c := graphql.NewClient(srhtEndpoint, hc)
	return &srhtForge{c: c}
}

// srhtVis maps to SourceHut's Visibility GraphQL enum
type srhtVis string

func (srhtVis) GetGraphQLType() string { return "Visibility" }

const (
	srhtPublic  srhtVis = "PUBLIC"
	srhtPrivate srhtVis = "PRIVATE"
)

func srhtVisOf(v config.Visibility) srhtVis {
	if v == config.Public {
		return srhtPublic
	}
	return srhtPrivate
}

func srhtIsExists(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "name") || strings.Contains(s, "repository")
}

func (g *srhtForge) Create(_ string, m Meta) error {
	var mut struct {
		CreateRepository struct {
			ID int
		} `graphql:"createRepository(name: $name, visibility: $visibility, description: $description)"`
	}
	vars := map[string]any{
		"name":        graphql.String(m.Name),
		"visibility":  srhtVisOf(m.Vis),
		"description": graphql.String(descOrEmpty(m.Desc)),
	}
	err := g.c.Mutate(context.Background(), &mut, vars)
	if err != nil {
		if srhtIsExists(err) {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *srhtForge) repoID(name string) (int, error) {
	var q struct {
		Me struct {
			Repository struct {
				ID int
			} `graphql:"repository(name: $name)"`
		}
	}
	vars := map[string]any{
		"name": graphql.String(name),
	}
	if err := g.c.Query(context.Background(), &q, vars); err != nil {
		return 0, err
	}
	if q.Me.Repository.ID == 0 {
		return 0, fmt.Errorf("sourcehut: could not find repository id")
	}
	return q.Me.Repository.ID, nil
}

type repoInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Visibility  srhtVis `json:"visibility"`
}

func (repoInput) GetGraphQLType() string { return "RepoInput" }

func (g *srhtForge) Update(_ string, m Meta) error {
	id, err := g.repoID(m.Name)
	if err != nil {
		return err
	}
	var mut struct {
		UpdateRepository struct {
			ID int
		} `graphql:"updateRepository(id: $id, input: $input)"`
	}
	vars := map[string]any{
		"id": graphql.Int(id),
		"input": repoInput{
			Name:        m.Name,
			Description: descOrEmpty(m.Desc),
			Visibility:  srhtVisOf(m.Vis),
		},
	}
	return g.c.Mutate(context.Background(), &mut, vars)
}

func (g *srhtForge) Archive(_, _ string, _ bool) error {
	return fmt.Errorf("sourcehut does not support archive via api")
}

func (g *srhtForge) Delete(_ string, name string) error {
	id, err := g.repoID(name)
	if err != nil {
		return err
	}
	var mut struct {
		DeleteRepository struct {
			ID int
		} `graphql:"deleteRepository(id: $id)"`
	}
	vars := map[string]any{
		"id": graphql.Int(id),
	}
	return g.c.Mutate(context.Background(), &mut, vars)
}

func (g *srhtForge) List(_ string) ([]string, error) {
	var q struct {
		Me struct {
			Repositories struct {
				Results []struct {
					Name string
				}
			}
		}
	}
	if err := g.c.Query(context.Background(), &q, nil); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(q.Me.Repositories.Results))
	for _, r := range q.Me.Repositories.Results {
		names = append(names, r.Name)
	}
	return names, nil
}

func (g *srhtForge) Sync(user string, m Meta) error {
	err := g.Create(user, m)
	if err == nil {
		return nil
	}
	if err != ErrExists {
		return err
	}
	// sourcehut has no archive support, just update
	return g.Update(user, m)
}
