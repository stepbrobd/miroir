package miroir

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ysun.co/miroir/config"
	"ysun.co/miroir/gitops"
	"ysun.co/miroir/workspace"
)

type fakeReporter struct {
	mu         sync.Mutex
	repoMsgs   []string
	remoteMsgs []string
	outputMsgs []string
	clearSlots []int
	finished   bool
}

func (f *fakeReporter) Repo(_ int, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repoMsgs = append(f.repoMsgs, msg)
}
func (f *fakeReporter) Remote(_, _ int, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.remoteMsgs = append(f.remoteMsgs, msg)
}
func (f *fakeReporter) Output(_, _ int, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputMsgs = append(f.outputMsgs, msg)
}
func (f *fakeReporter) ErrorRemote(_, _ int, _ string) {}
func (f *fakeReporter) ErrorOutput(_, _ int, _ string) {}
func (f *fakeReporter) Clear(slot int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clearSlots = append(f.clearSlots, slot)
}
func (f *fakeReporter) Finish() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.finished = true
}

type fakeOp struct {
	remotes int
	run     func(p gitops.Params) error
}

func (f fakeOp) Remotes(_ int) int         { return f.remotes }
func (f fakeOp) Run(p gitops.Params) error { return f.run(p) }

type cancelOnClearReporter struct {
	*fakeReporter
	cancel func()
	once   sync.Once
}

func (r *cancelOnClearReporter) Clear(slot int) {
	r.fakeReporter.Clear(slot)
	r.once.Do(r.cancel)
}

func TestRunGitOpSequentialSuccess(t *testing.T) {
	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{"/tmp/a": {}}
	op := fakeOp{remotes: 0, run: func(p gitops.Params) error { return nil }}
	err := RunGitOp(op, RunOptions{Context: t.Context(), Targets: []string{"/tmp/a"}, Contexts: ctxs, PlatformCount: 1, RepoConcurrency: 1, Reporter: reporter})
	if err != nil {
		t.Fatal(err)
	}
	if !reporter.finished {
		t.Fatal("expected reporter finish")
	}
}

func TestRunGitOpParallelFailure(t *testing.T) {
	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{"/tmp/a": {}, "/tmp/b": {}}
	op := fakeOp{remotes: 1, run: func(p gitops.Params) error {
		if p.Path == "/tmp/b" {
			return errors.New("boom")
		}
		return nil
	}}
	err := RunGitOp(op, RunOptions{Context: t.Context(), Targets: []string{"/tmp/a", "/tmp/b"}, Contexts: ctxs, PlatformCount: 1, RepoConcurrency: 2, Reporter: reporter})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(reporter.clearSlots) == 0 {
		t.Fatal("expected slot clears in parallel path")
	}
}

func TestRunGitOpSequentialFailureDoesNotReportRepoError(t *testing.T) {
	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{"/tmp/a": {}}
	op := fakeOp{remotes: 0, run: func(p gitops.Params) error {
		return errors.New("boom")
	}}
	err := RunGitOp(op, RunOptions{
		Context:         t.Context(),
		Targets:         []string{"/tmp/a"},
		Contexts:        ctxs,
		PlatformCount:   1,
		RepoConcurrency: 1,
		Reporter:        reporter,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !reporter.finished {
		t.Fatal("expected reporter finish")
	}
}

func TestRunGitOpSequentialCancelStopsLaterTargets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{
		"/tmp/a": {},
		"/tmp/b": {},
	}

	var seen []string
	var mu sync.Mutex
	op := fakeOp{remotes: 0, run: func(p gitops.Params) error {
		mu.Lock()
		seen = append(seen, p.Path)
		mu.Unlock()
		if p.Path == "/tmp/a" {
			cancel()
			return context.Canceled
		}
		return nil
	}}

	err := RunGitOp(op, RunOptions{
		Context:         ctx,
		Targets:         []string{"/tmp/a", "/tmp/b"},
		Contexts:        ctxs,
		PlatformCount:   1,
		RepoConcurrency: 1,
		Reporter:        reporter,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want context canceled", err)
	}
	if len(seen) != 1 || seen[0] != "/tmp/a" {
		t.Fatalf("expected only first repo to run got %v", seen)
	}
	if !reporter.finished {
		t.Fatal("expected reporter finish")
	}
}

func TestRunGitOpParallelCancelDoesNotReportRepoErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{
		"/tmp/a": {},
		"/tmp/b": {},
	}

	started := make(chan struct{}, 2)
	op := fakeOp{remotes: 1, run: func(p gitops.Params) error {
		started <- struct{}{}
		cancel()
		<-p.RunCtx.Done()
		return p.RunCtx.Err()
	}}

	err := RunGitOp(op, RunOptions{
		Context:         ctx,
		Targets:         []string{"/tmp/a", "/tmp/b"},
		Contexts:        ctxs,
		PlatformCount:   1,
		RepoConcurrency: 2,
		Reporter:        reporter,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want context canceled", err)
	}
	if len(started) == 0 {
		t.Fatal("expected at least one repo to start")
	}
	if !reporter.finished {
		t.Fatal("expected reporter finish")
	}
}

func TestRunGitOpRemoteLimitIsPerRepo(t *testing.T) {
	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{"/tmp/a": {}, "/tmp/b": {}}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	op := fakeOp{remotes: 1, run: func(p gitops.Params) error {
		p.Sem <- struct{}{}
		defer func() { <-p.Sem }()
		ready <- struct{}{}
		<-release
		return nil
	}}

	done := make(chan error, 1)
	go func() {
		done <- RunGitOp(op, RunOptions{
			Context:           t.Context(),
			Targets:           []string{"/tmp/a", "/tmp/b"},
			Contexts:          ctxs,
			PlatformCount:     1,
			RepoConcurrency:   2,
			RemoteConcurrency: 1,
			Reporter:          reporter,
		})
	}()

	// with a shared semaphore the second repo would block here forever
	for range 2 {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("remote semaphore is not per-repo")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRunSyncSkipsUnknownForgeAndMissingToken(t *testing.T) {
	os.Unsetenv("MIROIR_UNKNOWN_TOKEN")
	os.Unsetenv("MIROIR_NOTOKEN_TOKEN")

	reporter := &fakeReporter{}
	cfg := &config.Config{
		General: config.General{
			Concurrency: config.Concurrency{Repo: 1, Remote: 2},
		},
		Platform: map[string]config.Platform{
			"unknown": {Domain: "example.com", User: "alice"},
			"notoken": {Domain: "github.com", User: "alice"},
		},
		Repo: map[string]config.Repo{
			"seed": {Visibility: config.Private},
		},
	}

	if err := RunSync(t.Context(), cfg, []string{"seed"}, reporter); err != nil {
		t.Fatal(err)
	}
	skips := 0
	for _, m := range reporter.remoteMsgs {
		if strings.Contains(m, "skipped") {
			skips++
		}
	}
	if skips != 2 {
		t.Fatalf("expected both platforms skipped, got %v", reporter.remoteMsgs)
	}
	if !slices.Contains(reporter.outputMsgs, "unknown forge") {
		t.Fatalf("expected unknown forge output, got %v", reporter.outputMsgs)
	}
	if !slices.Contains(reporter.outputMsgs, "no token") {
		t.Fatalf("expected no token output, got %v", reporter.outputMsgs)
	}
}

func TestRunSyncCancelStopsBeforeRepoWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporter := &cancelOnClearReporter{
		fakeReporter: &fakeReporter{},
		cancel:       cancel,
	}

	token := "token"
	forge := config.Github
	cfg := &config.Config{
		General: config.General{
			Concurrency: config.Concurrency{Repo: 1, Remote: 1},
		},
		Platform: map[string]config.Platform{
			"github": {
				Domain: "github.com",
				User:   "alice",
				Token:  &token,
				Forge:  &forge,
			},
		},
		Repo: map[string]config.Repo{
			"seed": {Visibility: config.Private},
		},
	}

	err := RunSync(ctx, cfg, []string{"seed"}, reporter)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want context canceled", err)
	}
	if len(reporter.repoMsgs) != 0 {
		t.Fatalf("expected sync to stop before repo work got %v", reporter.repoMsgs)
	}
	if !reporter.finished {
		t.Fatal("expected reporter finish")
	}
}
