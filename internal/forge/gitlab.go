package forge

import (
	"context"
	"fmt"

	gl "gitlab.com/gitlab-org/api/client-go"
	"ysun.co/miroir/internal/config"
)

type glForge struct {
	c *gl.Client
}

func newGitlab(token, domain string) (*glForge, error) {
	c, err := gl.NewClient(token, gl.WithBaseURL("https://"+domain))
	if err != nil {
		return nil, fmt.Errorf("gitlab client: %w", err)
	}
	return &glForge{c: c}, nil
}

func glVis(v config.Visibility) *gl.VisibilityValue {
	if v == config.Public {
		return gl.Ptr(gl.PublicVisibility)
	}
	return gl.Ptr(gl.PrivateVisibility)
}

func (g *glForge) Create(ctx context.Context, _ string, m Meta) error {
	desc := descOrEmpty(m.Desc)
	_, resp, err := g.c.Projects.CreateProject(&gl.CreateProjectOptions{
		Name:                 &m.Name,
		Description:          &desc,
		Visibility:           glVis(m.Vis),
		InitializeWithReadme: gl.Ptr(false),
	}, gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == 400 {
			return ErrExists
		}
		return err
	}
	return nil
}

func (g *glForge) Update(ctx context.Context, user string, m Meta) error {
	pid := user + "/" + m.Name
	desc := descOrEmpty(m.Desc)
	_, _, err := g.c.Projects.EditProject(pid, &gl.EditProjectOptions{
		Name:        &m.Name,
		Description: &desc,
		Visibility:  glVis(m.Vis),
	}, gl.WithContext(ctx))
	return err
}

func (g *glForge) Archive(ctx context.Context, user, name string, flag bool) error {
	pid := user + "/" + name
	if flag {
		_, _, err := g.c.Projects.ArchiveProject(pid, gl.WithContext(ctx))
		return err
	}
	_, _, err := g.c.Projects.UnarchiveProject(pid, gl.WithContext(ctx))
	return err
}

func (g *glForge) Delete(ctx context.Context, user, name string) error {
	pid := user + "/" + name
	_, err := g.c.Projects.DeleteProject(pid, nil, gl.WithContext(ctx))
	return err
}

func (g *glForge) List(ctx context.Context, _ string) ([]string, error) {
	owned := true
	projs, _, err := g.c.Projects.ListProjects(&gl.ListProjectsOptions{
		Owned:       &owned,
		ListOptions: gl.ListOptions{PerPage: 100},
	}, gl.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(projs))
	for _, p := range projs {
		names = append(names, p.Name)
	}
	return names, nil
}

func (g *glForge) Sync(ctx context.Context, user string, m Meta) error {
	err := g.Create(ctx, user, m)
	if err == nil {
		return nil
	}
	if err != ErrExists {
		return err
	}
	if err := g.Update(ctx, user, m); err != nil {
		return err
	}
	if m.Archived {
		return g.Archive(ctx, user, m.Name, true)
	}
	return nil
}
