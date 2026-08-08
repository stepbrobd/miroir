package gitops

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
func run(ctx context.Context, dir string, env []string, silent bool, onOutput func(string), args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
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
			return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
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
	scanErr := sc.Err()
	if scanErr != nil {
		// drain so the child cannot block on a full pipe before Wait
		_, _ = io.Copy(io.Discard, pr)
	}
	pr.Close()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return fmt.Errorf("reading git output: %w", scanErr)
	}
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				return fmt.Errorf("git %s killed by signal %d",
					strings.Join(args, " "), ws.Signal())
			}
			return fmt.Errorf("git %s exited with code %d",
				strings.Join(args, " "), exitErr.ExitCode())
		}
		return waitErr
	}
	return nil
}

func repoName(path string) string {
	return filepath.Base(path)
}

func ensureRepo(path string) error {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a git repository", path)
	}
	return nil
}

func isDirty(ctx context.Context, dir string, env []string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git",
		"status",
		"--porcelain",
		"--untracked-files=normal",
		"--ignore-submodules=all",
	)
	cmd.Dir = dir
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(out) > 0, nil
}
