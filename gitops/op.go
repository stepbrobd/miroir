package git

import (
	"ysun.co/miroir/report"
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
	Disp  report.Reporter
	Slot  int
	Sem   chan struct{} // bounds concurrent remote operations
	Force bool
	Args  []string
}
