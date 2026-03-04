package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Exec runs an arbitrary command in the repo directory;
// remotes = 0: no display, runs sequentially with direct stdout
type Exec struct{}

func (Exec) Remotes(_ int) int { return 0 }

func (Exec) Run(p Params) error {
	name := repoName(p.Path)
	fmt.Printf("%s :: exec :: %s\n", name, strings.Join(p.Args, " "))
	if len(p.Args) == 0 {
		return nil
	}
	cmd := exec.Command(p.Args[0], p.Args[1:]...)
	cmd.Dir = p.Path
	cmd.Env = p.Ctx.Env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
