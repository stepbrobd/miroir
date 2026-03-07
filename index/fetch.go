package index

import (
	"bytes"
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
	path := repoPath(dir, r.Name, bare)
	if bare {
		return path, syncBareRepo(path, r, env)
	}
	return path, syncWorktreeRepo(path, r, env)
}

func repoPath(dir, name string, bare bool) string {
	if bare {
		return filepath.Join(dir, name+".git")
	}
	return filepath.Join(dir, name)
}

func syncBareRepo(path string, r Repo, env CmdEnv) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := initManagedBareRepo(path, env); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := ensureBareRepo(path, env); err != nil {
		return err
	}
	if err := ensureRemote(path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(path, env, r.Name); err != nil {
		return err
	}
	if err := setWebMetadata(path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(path, env); err != nil {
		return err
	}
	if err := setFetchRefspec(path, env, bareOriginFetchRefspec); err != nil {
		return err
	}
	if err := git(path, env, "fetch", "--prune", "origin"); err != nil {
		return err
	}
	return syncBareHeads(path, r.Branch, env)
}

func syncWorktreeRepo(path string, r Repo, env CmdEnv) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := cloneWorktreeRepo(path, r, env); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := ensureWorktreeRepo(path, env); err != nil {
		return err
	}
	if err := ensureRemote(path, env, "origin", r.URI); err != nil {
		return err
	}
	if err := setZoektName(path, env, r.Name); err != nil {
		return err
	}
	if err := setWebMetadata(path, env, r.WebURL, r.WebURLType); err != nil {
		return err
	}
	if err := setManagedMarker(path, env); err != nil {
		return err
	}
	log.Info("fetching", "repo", filepath.Base(path))
	return git(path, env, "fetch", "--prune", "origin")
}

func initManagedBareRepo(path string, env CmdEnv) error {
	log.Info("initializing", "repo", filepath.Base(path), "bare", true)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return git(path, env, "init", "--bare", path)
}

func cloneWorktreeRepo(path string, r Repo, env CmdEnv) error {
	log.Info("cloning", "repo", r.Name, "bare", false)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return git(path, env, "clone", "--branch", r.Branch, r.URI, path)
}

func ensureBareRepo(path string, env CmdEnv) error {
	out, err := gitOutput(path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "true" {
		return fmt.Errorf("%s is not a bare git repository", path)
	}
	return nil
}

func ensureWorktreeRepo(path string, env CmdEnv) error {
	out, err := gitOutput(path, env, "rev-parse", "--is-bare-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "false" {
		return fmt.Errorf("%s is not a non-bare git repository", path)
	}
	return nil
}

func ensureRemote(path string, env CmdEnv, name, uri string) error {
	current, ok, err := remoteURL(path, env, name)
	if err != nil {
		return err
	}
	if !ok {
		return git(path, env, "remote", "add", name, uri)
	}
	if current == uri {
		return nil
	}
	return git(path, env, "remote", "set-url", name, uri)
}

func setZoektName(path string, env CmdEnv, name string) error {
	return setRepoConfig(path, env, "zoekt.name", name)
}

func setWebMetadata(path string, env CmdEnv, webURL, webURLType string) error {
	if webURL == "" || webURLType == "" {
		if err := unsetRepoConfig(path, env, "zoekt.web-url"); err != nil {
			return err
		}
		return unsetRepoConfig(path, env, "zoekt.web-url-type")
	}
	if err := setRepoConfig(path, env, "zoekt.web-url", webURL); err != nil {
		return err
	}
	return setRepoConfig(path, env, "zoekt.web-url-type", webURLType)
}

func setManagedMarker(path string, env CmdEnv) error {
	return setRepoConfig(path, env, "miroir.managed", "true")
}

func setRepoConfig(path string, env CmdEnv, key, value string) error {
	current, ok, err := repoConfig(path, env, key)
	if err != nil {
		return err
	}
	if ok && current == value {
		return nil
	}
	return git(path, env, "config", key, value)
}

func unsetRepoConfig(path string, env CmdEnv, key string) error {
	_, ok, err := repoConfig(path, env, key)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return git(path, env, "config", "--unset-all", key)
}

func remoteURL(path string, env CmdEnv, name string) (string, bool, error) {
	return repoConfig(path, env, "remote."+name+".url")
}

func repoConfig(path string, env CmdEnv, key string) (string, bool, error) {
	cmd := gitCmd(path, env, "config", "--get", key)
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

func setFetchRefspec(path string, env CmdEnv, refspec string) error {
	_ = git(path, env, "config", "--unset-all", "remote.origin.fetch")
	return git(path, env, "config", "--add", "remote.origin.fetch", refspec)
}

func syncBareHeads(path, branch string, env CmdEnv) error {
	remoteHeads, err := listRefs(path, env, "refs/remotes/origin", 3)
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
		hash, err := resolveRef(path, env, "refs/remotes/origin/"+name)
		if err != nil {
			return err
		}
		if err := git(path, env, "update-ref", "refs/heads/"+name, hash); err != nil {
			return err
		}
	}
	if err := git(path, env, "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return err
	}

	localHeads, err := listRefs(path, env, "refs/heads", 2)
	if err != nil {
		return err
	}
	for _, name := range localHeads {
		if slices.Contains(remoteHeads, name) {
			continue
		}
		if err := git(path, env, "update-ref", "-d", "refs/heads/"+name); err != nil {
			return err
		}
	}
	return nil
}

func listRefs(path string, env CmdEnv, prefix string, strip int) ([]string, error) {
	out, err := gitOutput(path, env,
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
	out, err := gitOutput(path, env, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// git runs a git command, logging stderr through charm log
// dir is used as working directory only if it exists
func git(dir string, env CmdEnv, args ...string) error {
	cmd := gitCmd(dir, env, args...)
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
	cmd := gitCmd(dir, env, args...)
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

func gitCmd(dir string, env CmdEnv, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}
