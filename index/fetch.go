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
	"strings"

	"github.com/charmbracelet/log"
)

// repo describes a managed repo to keep updated
// indexName is the served zoekt repository name and must be set
type Repo struct {
	Name       string
	IndexName  string
	URI        string // origin URI
	Branch     string
	WebURL     string
	WebURLType string
}

type CmdEnv []string

const bareOriginFetchRefspec = "+refs/heads/*:refs/remotes/origin/*"

// fetch clones or fetches a managed repo under the parent directory dir
// returns the full path to the repo on disk
func Fetch(ctx context.Context, dir string, r Repo, bare bool, env CmdEnv) (string, error) {
	path := repoPath(dir, r.Name, bare)
	if bare {
		return path, syncBareRepo(ctx, path, r, env)
	}
	return path, syncWorktreeRepo(ctx, path, r, env)
}

func repoPath(dir, name string, bare bool) string {
	if bare {
		return filepath.Join(dir, name+".git")
	}
	return filepath.Join(dir, name)
}

func syncBareRepo(ctx context.Context, path string, r Repo, env CmdEnv) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return bootstrapBareRepo(ctx, path, r, env)
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	if err := ensureBareRepo(ctx, path, env); err != nil {
		return err
	}
	if err := ensureRemote(ctx, path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(ctx, path, env, r.IndexName); err != nil {
		return err
	}
	if err := setWebMetadata(ctx, path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(ctx, path, env); err != nil {
		return err
	}
	if err := setFetchRefspec(ctx, path, env, bareOriginFetchRefspec); err != nil {
		return err
	}
	if err := git(ctx, path, env, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	return syncBareHeads(ctx, path, r.Branch, env)
}

func syncWorktreeRepo(ctx context.Context, path string, r Repo, env CmdEnv) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return bootstrapWorktreeRepo(ctx, path, r, env)
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	if err := ensureWorktreeRepo(ctx, path, env); err != nil {
		return err
	}
	if err := ensureRemote(ctx, path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(ctx, path, env, r.IndexName); err != nil {
		return err
	}
	if err := setWebMetadata(ctx, path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(ctx, path, env); err != nil {
		return err
	}
	log.Info("fetching", "repo", filepath.Base(path))
	return git(ctx, path, env, "fetch", "--prune", "origin")
}

func bootstrapBareRepo(ctx context.Context, path string, r Repo, env CmdEnv) (err error) {
	tmp, err := tempRepoDir(path)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		_ = os.RemoveAll(tmp)
	}()

	if err := initManagedBareRepo(ctx, tmp, env); err != nil {
		return err
	}
	if err := ensureRemote(ctx, tmp, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(ctx, tmp, env, r.IndexName); err != nil {
		return err
	}
	if err := setWebMetadata(ctx, tmp, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(ctx, tmp, env); err != nil {
		return err
	}
	if err := setFetchRefspec(ctx, tmp, env, bareOriginFetchRefspec); err != nil {
		return err
	}
	if err := git(ctx, tmp, env, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	if err := syncBareHeads(ctx, tmp, r.Branch, env); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func bootstrapWorktreeRepo(ctx context.Context, path string, r Repo, env CmdEnv) (err error) {
	tmp, err := tempRepoDir(path)
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		_ = os.RemoveAll(tmp)
	}()

	if err := cloneWorktreeRepo(ctx, tmp, r, env); err != nil {
		return err
	}
	if err := ensureWorktreeRepo(ctx, tmp, env); err != nil {
		return err
	}
	if err := ensureRemote(ctx, tmp, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(ctx, tmp, env, r.IndexName); err != nil {
		return err
	}
	if err := setWebMetadata(ctx, tmp, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(ctx, tmp, env); err != nil {
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

func initManagedBareRepo(ctx context.Context, path string, env CmdEnv) error {
	log.Info("initializing", "repo", filepath.Base(path), "bare", true)
	return git(ctx, path, env, "init", "--bare", path)
}

func cloneWorktreeRepo(ctx context.Context, path string, r Repo, env CmdEnv) error {
	log.Info("cloning", "repo", r.Name, "bare", false)
	return git(ctx, path, env, "clone", "--branch", r.Branch, r.URI, path)
}

func ensureBareRepo(ctx context.Context, path string, env CmdEnv) error {
	out, err := gitOutput(ctx, path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "true" {
		return fmt.Errorf("%s is not a bare git repository", path)
	}
	return nil
}

func ensureWorktreeRepo(ctx context.Context, path string, env CmdEnv) error {
	out, err := gitOutput(ctx, path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "false" {
		return fmt.Errorf("%s is not a non-bare git repository", path)
	}
	return nil
}

func ensureRemote(ctx context.Context, path string, env CmdEnv, name, uri string) error {
	current, ok, err := remoteURL(ctx, path, env, name)
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

func setZoektName(ctx context.Context, path string, env CmdEnv, name string) error {
	return setRepoConfig(ctx, path, env, "zoekt.name", name)
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

func setManagedMarker(ctx context.Context, path string, env CmdEnv) error {
	return setRepoConfig(ctx, path, env, "miroir.managed", "true")
}

func setRepoConfig(ctx context.Context, path string, env CmdEnv, key, value string) error {
	current, ok, err := repoConfig(ctx, path, env, key)
	if err != nil {
		return err
	}
	if ok && current == value {
		return nil
	}
	return git(ctx, path, env, "config", key, value)
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

func remoteURL(ctx context.Context, path string, env CmdEnv, name string) (string, bool, error) {
	return repoConfig(ctx, path, env, "remote."+name+".url")
}

func repoConfig(ctx context.Context, path string, env CmdEnv, key string) (string, bool, error) {
	cmd := gitCmd(ctx, path, env, "config", "--get", key)
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
		log.Error("git", "args", []string{"config", "--get", key}, "stderr", stderr.String())
	}
	return "", false, fmt.Errorf("git config: %w", err)
}

func setFetchRefspec(ctx context.Context, path string, env CmdEnv, refspec string) error {
	return git(ctx, path, env, "config", "--replace-all", "remote.origin.fetch", refspec)
}

func syncBareHeads(ctx context.Context, path, branch string, env CmdEnv) error {
	remoteHeads, err := listRefs(ctx, path, env, "refs/remotes/origin", 3)
	if err != nil {
		return err
	}
	remoteHeads = slices.DeleteFunc(remoteHeads, func(name string) bool {
		return name == "HEAD"
	})
	if !slices.Contains(remoteHeads, branch) {
		return fmt.Errorf("origin branch %s not found", branch)
	}

	for _, name := range remoteHeads {
		hash, err := resolveRef(ctx, path, env, "refs/remotes/origin/"+name)
		if err != nil {
			return err
		}
		if err := git(ctx, path, env, "update-ref", "refs/heads/"+name, hash); err != nil {
			return err
		}
	}
	if err := git(ctx, path, env, "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return err
	}

	localHeads, err := listRefs(ctx, path, env, "refs/heads", 2)
	if err != nil {
		return err
	}
	for _, name := range localHeads {
		if slices.Contains(remoteHeads, name) {
			continue
		}
		if err := git(ctx, path, env, "update-ref", "-d", "refs/heads/"+name); err != nil {
			return err
		}
	}
	return nil
}

func listRefs(ctx context.Context, path string, env CmdEnv, prefix string, strip int) ([]string, error) {
	out, err := gitOutput(ctx, path, env,
		"for-each-ref",
		fmt.Sprintf("--format=%%(refname:strip=%d)", strip),
		prefix,
	)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return slices.DeleteFunc(lines, func(line string) bool {
		return strings.TrimSpace(line) == ""
	}), nil
}

func resolveRef(ctx context.Context, path string, env CmdEnv, ref string) (string, error) {
	out, err := gitOutput(ctx, path, env, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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
		return fmt.Errorf("git %s: %w", args[0], err)
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
		return "", fmt.Errorf("git %s: %w", args[0], err)
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
