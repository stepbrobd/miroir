package git

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"ysun.co/miroir/workspace"
)

func Available() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not available in PATH")
	}
	return nil
}

// stdout and stderr are merged and delivered line-by-line via onOutput
// when silent is true, output is suppressed but stderr is captured on failure
func run(dir string, env []string, silent bool, onOutput func(string), args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env

	if silent {
		var stderr bytes.Buffer
		cmd.Stdout = nil
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if stderr.Len() > 0 {
				return fmt.Errorf("git %s: %w: %s",
					strings.Join(args, " "), err, stderr.String())
			}
			return err
		}
		return nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return err
	}
	pw.Close()

	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		line := sc.Text()
		if len(line) > 0 && onOutput != nil {
			onOutput(line)
		}
	}
	pr.Close()
	if err := sc.Err(); err != nil {
		return fmt.Errorf("reading git output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				return fmt.Errorf("git %s killed by signal %d",
					strings.Join(args, " "), ws.Signal())
			}
			return fmt.Errorf("git %s exited with code %d",
				strings.Join(args, " "), exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func remoteIndex(ctx *workspace.Context, name string) int {
	for i, r := range ctx.Push {
		if r.Name == name {
			return i
		}
	}
	return -1
}

func repoName(path string) string {
	return filepath.Base(path)
}

func ensureRepo(path string) error {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("fatal: %s is not a git repository", path)
	}
	return nil
}

// fail-safe: returns true (dirty) on error to prevent unsafe operations
func isDirty(dir string, env []string) bool {
	dirty := false
	err := run(dir, env, false, func(_ string) { dirty = true }, "status", "--porcelain")
	if err != nil {
		return true
	}
	return dirty
}
