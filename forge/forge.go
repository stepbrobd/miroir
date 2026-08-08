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

// forge is the per-platform reconciliation entry point
// sync is create-or-update with archive handling where supported
type Forge interface {
	Sync(ctx context.Context, user string, m Meta) error
}

// create helpers return ErrExists when the repo already exists
var ErrExists = errors.New("already exists")

func Dispatch(f config.Forge, token, domain string) (Forge, error) {
	switch f {
	case config.Github:
		return newGithub(token, domain)
	case config.Gitlab:
		return newGitlab(token, domain)
	case config.Codeberg:
		return newCodeberg(token, domain)
	case config.Sourcehut:
		return newSourcehut(token, domain), nil
	default:
		return nil, fmt.Errorf("unknown forge %d", int(f))
	}
}

func descOrEmpty(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
