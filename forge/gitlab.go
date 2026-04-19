package forge

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	gl "gitlab.com/gitlab-org/api/client-go"
	"ysun.co/miroir/config"
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
		InitializeWithReadme: new(false),
	}, gl.WithContext(ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusBadRequest &&
			strings.Contains(err.Error(), "has already been taken") {
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

// gitlab returns 400 when archive/unarchive is a no-op
func glIsAlreadyArchived(resp *gl.Response, err error) bool {
	return resp != nil && resp.StatusCode == http.StatusBadRequest && err != nil &&
		(strings.Contains(err.Error(), "already archived") ||
			strings.Contains(err.Error(), "already unarchived"))
}

func (g *glForge) Archive(ctx context.Context, user, name string, flag bool) error {
	pid := user + "/" + name
	if flag {
		_, resp, err := g.c.Projects.ArchiveProject(pid, gl.WithContext(ctx))
		if glIsAlreadyArchived(resp, err) {
			return nil
		}
		return err
	}
	_, resp, err := g.c.Projects.UnarchiveProject(pid, gl.WithContext(ctx))
	if glIsAlreadyArchived(resp, err) {
		return nil
	}
	return err
}

func (g *glForge) Delete(ctx context.Context, user, name string) error {
	pid := user + "/" + name
	_, err := g.c.Projects.DeleteProject(pid, nil, gl.WithContext(ctx))
	return err
}

func (g *glForge) List(ctx context.Context, _ string) ([]string, error) {
	owned := true
	opt := &gl.ListProjectsOptions{
		Owned:       &owned,
		ListOptions: gl.ListOptions{PerPage: 100},
	}
	var names []string
	for {
		projs, resp, err := g.c.Projects.ListProjects(opt, gl.WithContext(ctx))
		if err != nil {
			return nil, err
		}
		for _, p := range projs {
			names = append(names, p.Name)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return names, nil
}

// update does not set archived so gitlab needs a separate archive call
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
	return g.Archive(ctx, user, m.Name, m.Archived)
}
