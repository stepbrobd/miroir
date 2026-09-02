package gitops

import (
	"context"

	"ysun.co/miroir/workspace"
)

type Reporter interface {
	Repo(slot int, msg string)
	Remote(slot, j int, msg string)
	Output(slot, j int, msg string)
	ErrorRemote(slot, j int, msg string)
	ErrorOutput(slot, j int, msg string)
	Clear(slot int)
	Finish()
}

// Op is one git operation run per target repository
type Op interface {
	// Remotes returns the number of display lines needed per repo slot
	// 0 means exec sequentially
	// 1 means origin only
	// n means all remotes
	Remotes(n int) int
	Run(p Params) error
}

// Params carries one repository run
// RunCtx must be non-nil
type Params struct {
	RunCtx context.Context
	Path   string
	Ctx    *workspace.Context
	Disp   Reporter
	Slot   int
	Sem    chan struct{} // bounds concurrent remote operations
	Force  bool
	Args   []string
}
