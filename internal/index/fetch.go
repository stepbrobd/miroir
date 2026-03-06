package index

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"
)

// Repo represents a managed repo to keep updated.
type Repo struct {
	Name   string
	URI    string // origin URI
	Branch string
}

// Fetch clones (if missing) or fetches (if present) a managed repo.
// dir is the parent directory. bare controls clone mode.
// returns the full path to the repo on disk.
func Fetch(dir string, r Repo, bare bool) (string, error) {
	var path string
	if bare {
		path = filepath.Join(dir, r.Name+".git")
	} else {
		path = filepath.Join(dir, r.Name)
	}

	_, err := os.Stat(path)
	switch {
	case err == nil:
		return path, fetchRepo(path)
	case os.IsNotExist(err):
		return path, cloneRepo(path, r, bare)
	default:
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
}

func cloneRepo(path string, r Repo, bare bool) error {
	log.Info("cloning", "repo", r.Name, "bare", bare)
	args := []string{"clone"}
	if bare {
		args = append(args, "--bare")
	}
	args = append(args, r.URI, path)
	if err := git(path, args...); err != nil {
		return err
	}
	// bare clones lack a fetch refspec, so git fetch is a no-op by default.
	// set one so subsequent fetches update local heads.
	if bare {
		return git(path, "config", "remote.origin.fetch", "+refs/*:refs/*")
	}
	return nil
}

func fetchRepo(path string) error {
	log.Info("fetching", "repo", filepath.Base(path))
	return git(path, "fetch", "origin")
}

// git runs a git command, logging stderr output through charm log.
// dir is used as working directory only if it exists (clone creates it).
func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			log.Error("git", "args", args, "stderr", stderr.String())
		}
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}
