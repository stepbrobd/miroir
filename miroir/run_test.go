package miroir

import (
	"errors"
	"testing"

	git "ysun.co/miroir/gitops"
	"ysun.co/miroir/workspace"
)

type fakeReporter struct {
	repoMsgs   []string
	errorMsgs  []string
	clearSlots []int
	finished   bool
}

func (f *fakeReporter) Repo(_ int, msg string)         { f.repoMsgs = append(f.repoMsgs, msg) }
func (f *fakeReporter) Remote(_, _ int, _ string)      {}
func (f *fakeReporter) Output(_, _ int, _ string)      {}
func (f *fakeReporter) Error(_ int, msg string)        { f.errorMsgs = append(f.errorMsgs, msg) }
func (f *fakeReporter) ErrorRemote(_, _ int, _ string) {}
func (f *fakeReporter) ErrorOutput(_, _ int, _ string) {}
func (f *fakeReporter) Clear(slot int)                 { f.clearSlots = append(f.clearSlots, slot) }
func (f *fakeReporter) Finish()                        { f.finished = true }

type fakeOp struct {
	remotes int
	run     func(p git.Params) error
}

func (f fakeOp) Remotes(_ int) int      { return f.remotes }
func (f fakeOp) Run(p git.Params) error { return f.run(p) }

func TestRunGitOpSequentialSuccess(t *testing.T) {
	reporter := &fakeReporter{}
	ctxs := map[string]*workspace.Context{"/tmp/a": {}}
	op := fakeOp{remotes: 0, run: func(p git.Params) error { return nil }}
	err := RunGitOp(op, RunOptions{Targets: []string{"/tmp/a"}, Contexts: ctxs, PlatformCount: 1, RepoConcurrency: 1, Reporter: reporter})
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
	op := fakeOp{remotes: 1, run: func(p git.Params) error {
		if p.Path == "/tmp/b" {
			return errors.New("boom")
		}
		return nil
	}}
	err := RunGitOp(op, RunOptions{Targets: []string{"/tmp/a", "/tmp/b"}, Contexts: ctxs, PlatformCount: 1, RepoConcurrency: 2, Reporter: reporter})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(reporter.clearSlots) == 0 {
		t.Fatal("expected slot clears in parallel path")
	}
	if len(reporter.errorMsgs) == 0 {
		t.Fatal("expected reported errors")
	}
}
