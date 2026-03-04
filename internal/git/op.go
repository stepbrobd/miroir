package git

import (
	"ysun.co/miroir/internal/context"
	"ysun.co/miroir/internal/display"
)

// Remotes returns display lines needed per repo slot:
// 0 = exec (sequential), 1 = origin only, n = all remotes
type Op interface {
	Remotes(n int) int
	Run(p Params) error
}

type Params struct {
	Path  string
	Ctx   *context.Context
	Disp  *display.Display
	Slot  int
	Sem   chan struct{} // bounds concurrent remote operations
	Force bool
	Args  []string
}
