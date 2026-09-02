package gitops

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Exec runs sequentially with direct stdout and no remote fan-out
type Exec struct{}

func (Exec) Remotes(_ int) int { return 0 }

func (Exec) Run(p Params) error {
	if len(p.Args) == 0 {
		return errors.New("no command provided")
	}
	name := repoName(p.Path)
	fmt.Printf("%s :: exec :: %s\n", name, strings.Join(p.Args, " "))
	cmd := exec.CommandContext(p.RunCtx, p.Args[0], p.Args[1:]...)
	cmd.Dir = p.Path
	cmd.Env = p.Ctx.Env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
