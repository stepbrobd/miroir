package index

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
)

// Repo describes a managed repo to keep updated
// IndexName is the served zoekt repository name and must be set
type Repo struct {
	Name       string
	IndexName  string
	URI        string // origin URI
	Branch     string
	WebURL     string
	WebURLType string
}

type CmdEnv []string

// a bare mirror keeps origin heads directly under refs/heads, so prune
// drops what origin dropped and HEAD can point at any of them
const bareFetchRefspec = "+refs/heads/*:refs/heads/*"

// managedKey marks a repo dir as daemon-owned, cleanup only removes those
const managedKey = "miroir.managed"

// Fetch clones or fetches a managed repo under the parent directory dir
// returns the full path to the repo on disk
func Fetch(ctx context.Context, dir string, r Repo, bare bool, env CmdEnv) (string, error) {
	path := repoPath(dir, r.Name, bare)
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return path, bootstrap(ctx, path, r, bare, env)
	case err != nil:
		return path, fmt.Errorf("stat %s: %w", path, err)
	case !info.IsDir():
		return path, fmt.Errorf("%s is not a directory", path)
	}
	return path, update(ctx, path, r, bare, env)
}

func repoPath(dir, name string, bare bool) string {
	if bare {
		return filepath.Join(dir, name+".git")
	}
	return filepath.Join(dir, name)
}

// bootstrap builds the repo in a temp dir and renames it into place
// so a crash never leaves a half made repo at path
func bootstrap(ctx context.Context, path string, r Repo, bare bool, env CmdEnv) (err error) {
	tmp, err := tempRepoDir(path)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmp)
		}
	}()

	if bare {
		log.Info("initializing", "repo", r.Name, "bare", true)
		if err := git(ctx, tmp, env, "init", "--bare", tmp); err != nil {
			return err
		}
	} else {
		log.Info("cloning", "repo", r.Name, "bare", false)
		if err := git(ctx, tmp, env, "clone", "--branch", r.Branch, r.URI, tmp); err != nil {
			return err
		}
	}
	if err := update(ctx, tmp, r, bare, env); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func tempRepoDir(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
}

// update refreshes an existing managed repo in place
func update(ctx context.Context, path string, r Repo, bare bool, env CmdEnv) error {
	if err := ensureBare(ctx, path, env, bare); err != nil {
		return err
	}
	if err := configure(ctx, path, r, env); err != nil {
		return err
	}
	if bare {
		return mirror(ctx, path, r.Branch, env)
	}
	return fetchOrigin(ctx, path, env)
}

func ensureBare(ctx context.Context, path string, env CmdEnv, bare bool) error {
	out, err := gitOutput(ctx, path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != strconv.FormatBool(bare) {
		return fmt.Errorf("%s is not a git repository with bare=%t", path, bare)
	}
	return nil
}

// configure pins the origin url and the metadata zoekt reads from git config
func configure(ctx context.Context, path string, r Repo, env CmdEnv) error {
	if err := ensureRemote(ctx, path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setRepoConfig(ctx, path, env, "zoekt.name", r.IndexName); err != nil {
		return err
	}
	if err := setWebMetadata(ctx, path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	return setRepoConfig(ctx, path, env, managedKey, "true")
}

// mirror fetches origin heads into refs/heads and aims HEAD at branch
func mirror(ctx context.Context, path, branch string, env CmdEnv) error {
	if err := setRepoConfig(ctx, path, env, "remote.origin.fetch", bareFetchRefspec); err != nil {
		return err
	}
	if err := fetchOrigin(ctx, path, env, "--no-tags"); err != nil {
		return err
	}
	if err := dropTrackingRefs(ctx, path, env); err != nil {
		return err
	}
	return pointHead(ctx, path, branch, env)
}

// fetchOrigin keeps auto gc in the foreground
// a detached repack could delete a pack the indexer is still reading
func fetchOrigin(ctx context.Context, path string, env CmdEnv, extra ...string) error {
	args := append([]string{"-c", "gc.autoDetach=false", "fetch", "--prune"}, extra...)
	return git(ctx, path, env, append(args, "origin")...)
}

// dropTrackingRefs removes the refs/remotes/origin refs an older daemon kept
// prune never touches them since the refspec no longer maps there
func dropTrackingRefs(ctx context.Context, path string, env CmdEnv) error {
	refs, err := listRefs(ctx, path, env, "refs/remotes/origin")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if err := git(ctx, path, env, "update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return nil
}

// pointHead aims HEAD at branch once fetch has populated refs/heads
func pointHead(ctx context.Context, path, branch string, env CmdEnv) error {
	heads, err := listRefs(ctx, path, env, "refs/heads")
	if err != nil {
		return err
	}
	ref := "refs/heads/" + branch
	if !slices.Contains(heads, ref) {
		return fmt.Errorf("origin branch %s not found", branch)
	}
	return git(ctx, path, env, "symbolic-ref", "HEAD", ref)
}

// listRefs returns the full names of the refs under prefix
func listRefs(ctx context.Context, path string, env CmdEnv, prefix string) ([]string, error) {
	out, err := gitOutput(ctx, path, env, "for-each-ref", "--format=%(refname)", prefix)
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

func ensureRemote(ctx context.Context, path string, env CmdEnv, name, uri string) error {
	current, ok, err := repoConfig(ctx, path, env, "remote."+name+".url")
	if err != nil {
		return err
	}
	if !ok {
		return git(ctx, path, env, "remote", "add", name, uri)
	}
	if current == uri {
		return nil
	}
	return git(ctx, path, env, "remote", "set-url", name, uri)
}

func setWebMetadata(ctx context.Context, path string, env CmdEnv, webURL, webURLType string) error {
	if webURL == "" || webURLType == "" {
		if err := unsetRepoConfig(ctx, path, env, "zoekt.web-url"); err != nil {
			return err
		}
		return unsetRepoConfig(ctx, path, env, "zoekt.web-url-type")
	}
	if err := setRepoConfig(ctx, path, env, "zoekt.web-url", webURL); err != nil {
		return err
	}
	return setRepoConfig(ctx, path, env, "zoekt.web-url-type", webURLType)
}

// setRepoConfig writes key as a single value only when the stored values differ
func setRepoConfig(ctx context.Context, path string, env CmdEnv, key, value string) error {
	current, ok, err := repoConfig(ctx, path, env, key)
	if err != nil {
		return err
	}
	if ok && current == value {
		return nil
	}
	return git(ctx, path, env, "config", "--replace-all", key, value)
}

func unsetRepoConfig(ctx context.Context, path string, env CmdEnv, key string) error {
	_, ok, err := repoConfig(ctx, path, env, key)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return git(ctx, path, env, "config", "--unset-all", key)
}

// repoConfig reads every value of key joined by newlines
// ok is false when the key is unset
func repoConfig(ctx context.Context, path string, env CmdEnv, key string) (string, bool, error) {
	cmd := gitCmd(ctx, path, env, "config", "--get-all", key)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()), true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stdout.String()) == "" {
		return "", false, nil
	}
	if stderr.Len() > 0 {
		log.Error("git", "args", []string{"config", "--get-all", key}, "stderr", stderr.String())
	}
	return "", false, fmt.Errorf("git config: %w", err)
}

// git runs a git command in dir, logging stderr through charm log
func git(ctx context.Context, dir string, env CmdEnv, args ...string) error {
	cmd := gitCmd(ctx, dir, env, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			log.Error("git", "args", args, "stderr", stderr.String())
		}
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, env CmdEnv, args ...string) (string, error) {
	cmd := gitCmd(ctx, dir, env, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			log.Error("git", "args", args, "stderr", stderr.String())
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

func gitCmd(ctx context.Context, dir string, env CmdEnv, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}
