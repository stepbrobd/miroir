package git

import (
	"ysun.co/miroir/internal/display"
	"ysun.co/miroir/workspace"
)

// Remotes returns display lines needed per repo slot:
// 0 = exec (sequential), 1 = origin only, n = all remotes
type Op interface {
	Remotes(n int) int
	Run(p Params) error
}

type Params struct {
	Path  string
	Ctx   *workspace.Context
	Disp  *display.Display
	Slot  int
	Sem   chan struct{} // bounds concurrent remote operations
	Force bool
	Args  []string
}
