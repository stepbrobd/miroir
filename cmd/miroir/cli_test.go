package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildCLI(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bin := filepath.Join(t.TempDir(), "miroir")

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/miroir")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, out)
	}
	return bin
}

func writeCLIConfig(t *testing.T, dir, home string, withRepo bool) string {
	t.Helper()

	cfg := fmt.Sprintf(`
[general]
home = %q
branch = "main"

[general.concurrency]
repo = 1
remote = 0

[platform.origin]
origin = true
domain = "github.com"

[index]
listen = ":0"
database = %q
interval = 3600
`, home, filepath.Join(dir, "index"))
	if withRepo {
		cfg += `
[repo.alpha]
visibility = "private"
`
	}

	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runInterruptedCLI(t *testing.T, bin, dir string, args ...string) (int, string) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	err := cmd.Wait()
	if err == nil {
		return 0, out.String()
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("wait: %v", err)
	}
	return exitErr.ExitCode(), out.String()
}

func TestCLIExecInterruptReturns130(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}

	bin := buildCLI(t)
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	repo := filepath.Join(home, "alpha")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := writeCLIConfig(t, tmp, home, true)

	code, out := runInterruptedCLI(t, bin, repo, "-c", cfg, "exec", "--", "sleep", "30")
	if code != 130 {
		t.Fatalf("exit code = %d want 130\n%s", code, out)
	}
}

func TestCLIIndexInterruptReturns130(t *testing.T) {
	bin := buildCLI(t)
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := writeCLIConfig(t, tmp, home, false)

	code, out := runInterruptedCLI(t, bin, tmp, "-c", cfg, "index")
	if code != 130 {
		t.Fatalf("exit code = %d want 130\n%s", code, out)
	}
}
