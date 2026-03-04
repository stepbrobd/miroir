package git

import (
	"ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/display"
)

// Op defines a git operation that can be dispatched uniformly
// Remotes declares how many remote display lines the operation
// needs per repo slot: 0 = no display (exec), 1 = origin only
// (pull/init), n = all remotes (fetch/push)
type Op interface {
	Remotes(n int) int
	Run(p Params) error
}

// Params holds all arguments for a git operation
type Params struct {
	Path  string
	Ctx   *context.Context
	Disp  *display.Display
	Slot  int
	Sem   chan struct{} // semaphore for remote concurrency
	Force bool
	Args  []string
}
