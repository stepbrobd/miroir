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

// fetch clones or fetches a managed repo
// dir is the parent directory, bare controls clone mode
// returns the full path to the repo on disk
func Fetch(dir string, r Repo, bare bool, env CmdEnv) (string, error) {
	return FetchContext(context.Background(), dir, r, bare, env)
}

func FetchContext(ctx context.Context, dir string, r Repo, bare bool, env CmdEnv) (string, error) {
	path := repoPath(dir, r.Name, bare)
	if bare {
		return path, syncBareRepoContext(ctx, path, r, env)
	}
	return path, syncWorktreeRepoContext(ctx, path, r, env)
}

func repoPath(dir, name string, bare bool) string {
	if bare {
		return filepath.Join(dir, name+".git")
	}
	return filepath.Join(dir, name)
}

func (r Repo) servedName() string {
	if r.IndexName != "" {
		return r.IndexName
	}
	return r.Name
}

func syncBareRepoContext(ctx context.Context, path string, r Repo, env CmdEnv) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := initManagedBareRepoContext(ctx, path, env); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := ensureBareRepoContext(ctx, path, env); err != nil {
		return err
	}
	if err := ensureRemoteContext(ctx, path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektNameContext(ctx, path, env, r.servedName()); err != nil {
		return err
	}
	if err := setWebMetadataContext(ctx, path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarkerContext(ctx, path, env); err != nil {
		return err
	}
	if err := setFetchRefspecContext(ctx, path, env, bareOriginFetchRefspec); err != nil {
		return err
	}
	if err := gitContext(ctx, path, env, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	return syncBareHeadsContext(ctx, path, r.Branch, env)
}

func syncWorktreeRepoContext(ctx context.Context, path string, r Repo, env CmdEnv) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cloneWorktreeRepoContext(ctx, path, r, env); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := ensureWorktreeRepoContext(ctx, path, env); err != nil {
		return err
	}
	if err := ensureRemoteContext(ctx, path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektNameContext(ctx, path, env, r.servedName()); err != nil {
		return err
	}
	if err := setWebMetadataContext(ctx, path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarkerContext(ctx, path, env); err != nil {
		return err
	}
	log.Info("fetching", "repo", filepath.Base(path))
	return gitContext(ctx, path, env, "fetch", "--prune", "origin")
}

func initManagedBareRepoContext(ctx context.Context, path string, env CmdEnv) error {
	log.Info("initializing", "repo", filepath.Base(path), "bare", true)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return gitContext(ctx, path, env, "init", "--bare", path)
}

func cloneWorktreeRepoContext(ctx context.Context, path string, r Repo, env CmdEnv) error {
	log.Info("cloning", "repo", r.Name, "bare", false)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return gitContext(ctx, path, env, "clone", "--branch", r.Branch, r.URI, path)
}

func ensureBareRepoContext(ctx context.Context, path string, env CmdEnv) error {
	out, err := gitOutputContext(ctx, path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "true" {
		return fmt.Errorf("%s is not a bare git repository", path)
	}
	return nil
}

func ensureWorktreeRepoContext(ctx context.Context, path string, env CmdEnv) error {
	out, err := gitOutputContext(ctx, path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "false" {
		return fmt.Errorf("%s is not a non-bare git repository", path)
	}
	return nil
}

func ensureRemoteContext(ctx context.Context, path string, env CmdEnv, name, uri string) error {
	current, ok, err := remoteURLContext(ctx, path, env, name)
	if err != nil {
		return err
	}
	if !ok {
		return gitContext(ctx, path, env, "remote", "add", name, uri)
	}
	if current == uri {
		return nil
	}
	return gitContext(ctx, path, env, "remote", "set-url", name, uri)
}

func setZoektNameContext(ctx context.Context, path string, env CmdEnv, name string) error {
	return setRepoConfigContext(ctx, path, env, "zoekt.name", name)
}

func setWebMetadataContext(ctx context.Context, path string, env CmdEnv, webURL, webURLType string) error {
	if webURL == "" || webURLType == "" {
		if err := unsetRepoConfigContext(ctx, path, env, "zoekt.web-url"); err != nil {
			return err
		}
		return unsetRepoConfigContext(ctx, path, env, "zoekt.web-url-type")
	}
	if err := setRepoConfigContext(ctx, path, env, "zoekt.web-url", webURL); err != nil {
		return err
	}
	return setRepoConfigContext(ctx, path, env, "zoekt.web-url-type", webURLType)
}

func setManagedMarkerContext(ctx context.Context, path string, env CmdEnv) error {
	return setRepoConfigContext(ctx, path, env, "miroir.managed", "true")
}

func setRepoConfigContext(ctx context.Context, path string, env CmdEnv, key, value string) error {
	current, ok, err := repoConfigContext(ctx, path, env, key)
	if err != nil {
		return err
	}
	if ok && current == value {
		return nil
	}
	return gitContext(ctx, path, env, "config", key, value)
}

func unsetRepoConfigContext(ctx context.Context, path string, env CmdEnv, key string) error {
	_, ok, err := repoConfigContext(ctx, path, env, key)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return gitContext(ctx, path, env, "config", "--unset-all", key)
}

func remoteURLContext(ctx context.Context, path string, env CmdEnv, name string) (string, bool, error) {
	return repoConfigContext(ctx, path, env, "remote."+name+".url")
}

func repoConfig(path string, env CmdEnv, key string) (string, bool, error) {
	return repoConfigContext(context.Background(), path, env, key)
}

func repoConfigContext(ctx context.Context, path string, env CmdEnv, key string) (string, bool, error) {
	cmd := gitCmdContext(ctx, path, env, "config", "--get", key)
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

func setFetchRefspecContext(ctx context.Context, path string, env CmdEnv, refspec string) error {
	_ = gitContext(ctx, path, env, "config", "--unset-all", "remote.origin.fetch")
	return gitContext(ctx, path, env, "config", "--add", "remote.origin.fetch", refspec)
}

func syncBareHeadsContext(ctx context.Context, path, branch string, env CmdEnv) error {
	remoteHeads, err := listRefsContext(ctx, path, env, "refs/remotes/origin", 3)
	if err != nil {
		return err
	}
	remoteHeads = slices.DeleteFunc(remoteHeads, func(name string) bool {
		return name == "" || name == "HEAD"
	})
	if !slices.Contains(remoteHeads, branch) {
		return fmt.Errorf("origin branch %s not found", branch)
	}

	for _, name := range remoteHeads {
		hash, err := resolveRefContext(ctx, path, env, "refs/remotes/origin/"+name)
		if err != nil {
			return err
		}
		if err := gitContext(ctx, path, env, "update-ref", "refs/heads/"+name, hash); err != nil {
			return err
		}
	}
	if err := gitContext(ctx, path, env, "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return err
	}

	localHeads, err := listRefsContext(ctx, path, env, "refs/heads", 2)
	if err != nil {
		return err
	}
	for _, name := range localHeads {
		if slices.Contains(remoteHeads, name) {
			continue
		}
		if err := gitContext(ctx, path, env, "update-ref", "-d", "refs/heads/"+name); err != nil {
			return err
		}
	}
	return nil
}

func listRefs(path string, env CmdEnv, prefix string, strip int) ([]string, error) {
	return listRefsContext(context.Background(), path, env, prefix, strip)
}

func listRefsContext(ctx context.Context, path string, env CmdEnv, prefix string, strip int) ([]string, error) {
	out, err := gitOutputContext(ctx, path, env,
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

func resolveRef(path string, env CmdEnv, ref string) (string, error) {
	return resolveRefContext(context.Background(), path, env, ref)
}

func resolveRefContext(ctx context.Context, path string, env CmdEnv, ref string) (string, error) {
	out, err := gitOutputContext(ctx, path, env, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// git runs a git command, logging stderr through charm log
// dir is used as working directory only if it exists
func gitContext(ctx context.Context, dir string, env CmdEnv, args ...string) error {
	cmd := gitCmdContext(ctx, dir, env, args...)
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

func gitOutput(dir string, env CmdEnv, args ...string) (string, error) {
	return gitOutputContext(context.Background(), dir, env, args...)
}

func gitOutputContext(ctx context.Context, dir string, env CmdEnv, args ...string) (string, error) {
	cmd := gitCmdContext(ctx, dir, env, args...)
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

func gitCmdContext(ctx context.Context, dir string, env CmdEnv, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(contextOrBackground(ctx), "git", args...)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
