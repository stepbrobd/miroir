package forge

import (
	"context"
	"errors"
	"fmt"

	"ysun.co/miroir/config"
)

type Meta struct {
	Name     string
	Desc     *string
	Vis      config.Visibility
	Archived bool
}

// forge abstracts forge CRUD operations
// create returns ErrExists if the repo already exists
// archive may return ErrUnsupported on forges without archive API
// list returns all repos owned by the authenticated user
// sync is create-or-update with archive handling
type Forge interface {
	Create(ctx context.Context, user string, m Meta) error
	Update(ctx context.Context, user string, m Meta) error
	Archive(ctx context.Context, user, name string, flag bool) error
	Delete(ctx context.Context, user, name string) error
	List(ctx context.Context, user string) ([]string, error)
	Sync(ctx context.Context, user string, m Meta) error
}

var (
	ErrExists      = errors.New("already exists")
	ErrUnsupported = errors.New("not supported by this forge")
)

func Dispatch(f config.Forge, token, domain string) (Forge, error) {
	switch f {
	case config.Github:
		return newGithub(token), nil
	case config.Gitlab:
		return newGitlab(token, domain)
	case config.Codeberg:
		return newCodeberg(token)
	case config.Sourcehut:
		return newSourcehut(token), nil
	default:
		return nil, fmt.Errorf("unknown forge: %v", f)
	}
}

func descOrEmpty(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
