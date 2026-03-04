package forge

import (
	"fmt"

	"ysun.co/miroir/internal/config"
)

// Meta holds repository metadata for forge sync
type Meta struct {
	Name     string
	Desc     *string
	Vis      config.Visibility
	Archived bool
}

// Forge defines operations on a git hosting platform
type Forge interface {
	Create(user string, m Meta) error
	Update(user string, m Meta) error
	Archive(user, name string, flag bool) error
	Delete(user, name string) error
	List(user string) ([]string, error)
	Sync(user string, m Meta) error
}

// ErrExists is returned when a repo already exists on the forge
var ErrExists = fmt.Errorf("already exists")

// Dispatch returns the forge implementation for a given forge type
func Dispatch(f config.Forge, token string) (Forge, error) {
	switch f {
	case config.Github:
		return newGithub(token), nil
	case config.Gitlab:
		return newGitlab(token)
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
