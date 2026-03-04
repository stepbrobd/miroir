package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"ysun.co/miroir/internal/context"
)

// Available checks if git is in PATH
func Available() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git is not available in PATH")
	}
	return nil
}

// run executes a git command, routing combined stdout/stderr
// through onOutput; if silent, output is discarded
func run(dir string, env []string, silent bool, onOutput func(string), args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env

	if silent {
		cmd.Stdout = nil
		cmd.Stderr = nil
		return cmd.Run()
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

func remoteIndex(ctx *context.Context, name string) int {
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

func isDirty(dir string, env []string) bool {
	dirty := false
	err := run(dir, env, false, func(_ string) { dirty = true }, "status", "--porcelain")
	if err != nil {
		return false
	}
	return dirty
}
