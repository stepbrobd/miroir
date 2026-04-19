package forge

import (
	"context"
	"errors"
	"net/http"
	"strings"

	graphql "github.com/hasura/go-graphql-client"
	"ysun.co/miroir/config"
)

const srhtEndpoint = "https://git.sr.ht/query"

type srhtForge struct {
	c *graphql.Client
}

type srhtTransport struct {
	token string
	base  http.RoundTripper
}

func (t *srhtTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func newSourcehut(token string) *srhtForge {
	hc := &http.Client{
		Transport: &srhtTransport{token: token, base: http.DefaultTransport},
	}
	c := graphql.NewClient(srhtEndpoint, hc)
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

func (g *srhtForge) Create(ctx context.Context, _ string, m Meta) error {
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

func (g *srhtForge) Update(ctx context.Context, _ string, m Meta) error {
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

func (g *srhtForge) Archive(_ context.Context, _, _ string, _ bool) error {
	return ErrUnsupported
}

func (g *srhtForge) Delete(ctx context.Context, _ string, name string) error {
	id, err := g.repoID(ctx, name)
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
	return g.c.Mutate(ctx, &mut, vars)
}

// sourcehut GraphQL cursor-based pagination
func (g *srhtForge) List(ctx context.Context, _ string) ([]string, error) {
	type cursor struct {
		Results []struct {
			Name string
		}
		Cursor *string
	}
	var names []string
	var after *graphql.String
	for {
		var q struct {
			Me struct {
				Repositories cursor `graphql:"repositories(cursor: $cursor)"`
			}
		}
		vars := map[string]any{"cursor": after}
		if err := g.c.Query(ctx, &q, vars); err != nil {
			return nil, err
		}
		for _, r := range q.Me.Repositories.Results {
			names = append(names, r.Name)
		}
		if q.Me.Repositories.Cursor == nil || len(q.Me.Repositories.Results) == 0 {
			break
		}
		s := graphql.String(*q.Me.Repositories.Cursor)
		after = &s
	}
	return names, nil
}

func (g *srhtForge) Sync(ctx context.Context, user string, m Meta) error {
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
		return g.Create(ctx, user, m)
	}
	desc := descOrEmpty(m.Desc)
	if q.Me.Repository.Description == desc && q.Me.Repository.Visibility == srhtVisOf(m.Vis) {
		return nil
	}
	return g.Update(ctx, user, m)
}
