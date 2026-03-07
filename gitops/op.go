package gitops

import (
	"ysun.co/miroir/workspace"
)

type Reporter interface {
	Repo(slot int, msg string)
	Remote(slot, j int, msg string)
	Output(slot, j int, msg string)
	Error(slot int, msg string)
	ErrorRemote(slot, j int, msg string)
	ErrorOutput(slot, j int, msg string)
	Clear(slot int)
	Finish()
}

// remotes returns the number of display lines needed per repo slot
// 0 = exec sequentially 1 = origin only n = all remotes
type Op interface {
	Remotes(n int) int
	Run(p Params) error
}

type Params struct {
	Path  string
	Ctx   *workspace.Context
	Disp  Reporter
	Slot  int
	Sem   chan struct{} // bounds concurrent remote operations
	Force bool
	Args  []string
}
