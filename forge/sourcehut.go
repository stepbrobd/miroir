package forge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"ysun.co/miroir/config"
)

type srhtForge struct {
	c *graphql.Client
}

type srhtTransport struct {
	token string
	base  http.RoundTripper
}

func (t *srhtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

func newSourcehut(token, domain string) *srhtForge {
	hc := &http.Client{
		Transport: &srhtTransport{token: token, base: http.DefaultTransport},
	}
	c := graphql.NewClient(fmt.Sprintf("https://%s/query", domain), hc)
	return &srhtForge{c: c}
}

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
	return strings.Contains(s, "already exists") || strings.Contains(s, "already in use")
}

func (g *srhtForge) create(ctx context.Context, m Meta) error {
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
	err := g.c.Mutate(ctx, &mut, vars)
	if err != nil {
		if srhtIsExists(err) {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *srhtForge) repoID(ctx context.Context, name string) (int, error) {
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
	if err := g.c.Query(ctx, &q, vars); err != nil {
		return 0, err
	}
	if q.Me.Repository.ID == 0 {
		return 0, errors.New("sourcehut: could not find repository id")
	}
	return q.Me.Repository.ID, nil
}

// omits Name to avoid spurious "already exists" from the uniqueness
// check (updates are identified by id, not name)
type repoUpdateInput struct {
	Description string  `json:"description"`
	Visibility  srhtVis `json:"visibility"`
}

func (repoUpdateInput) GetGraphQLType() string { return "RepoInput" }

func (g *srhtForge) update(ctx context.Context, m Meta) error {
	id, err := g.repoID(ctx, m.Name)
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
		"input": repoUpdateInput{
			Description: descOrEmpty(m.Desc),
			Visibility:  srhtVisOf(m.Vis),
		},
	}
	return g.c.Mutate(ctx, &mut, vars)
}

func (g *srhtForge) Sync(ctx context.Context, _ string, m Meta) error {
	var q struct {
		Me struct {
			Repository struct {
				ID          int
				Description string
				Visibility  srhtVis
			} `graphql:"repository(name: $name)"`
		}
	}
	vars := map[string]any{"name": graphql.String(m.Name)}
	if err := g.c.Query(ctx, &q, vars); err != nil {
		return err
	}
	if q.Me.Repository.ID == 0 {
		return g.create(ctx, m)
	}
	desc := descOrEmpty(m.Desc)
	if q.Me.Repository.Description == desc && q.Me.Repository.Visibility == srhtVisOf(m.Vis) {
		return nil
	}
	return g.update(ctx, m)
}
