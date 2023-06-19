package runnable

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/magdyamr542/reloader/config"
)

// Runnable is something that can run on the machine like a command.
type Runnable interface {
	// Run runs the executable. This starts and waits.
	Run() error
	// Start starts running the executable but doesn't wait for it.
	Start() error
	// Wait blocks till the executable is done.
	Wait() error
}

// Creator is a function that returns a Runnable.
type Creator func(ctx context.Context, command config.CommandWithDir) Runnable

type osCmd struct {
	cmd *exec.Cmd
}

func NewCmd(ctx context.Context, command config.CommandWithDir) Runnable {
	c := osCmd{cmd: newCmd(ctx, command)}
	return &c
}

func (o *osCmd) Run() error {
	return o.cmd.Run()
}

func (o *osCmd) Start() error {
	return o.cmd.Start()
}

func (o *osCmd) Wait() error {
	return o.cmd.Wait()
}

func newCmd(ctx context.Context, command config.CommandWithDir) *exec.Cmd {
	parts := strings.Split(command.Command, " ")
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = command.BaseDir

	// Build the envs for the command
	envs := os.Environ()
	if command.Env != nil {
		for key, value := range command.Env {
			envs = append(envs, fmt.Sprintf("%s=%s", key, value))
		}
	}
	cmd.Env = envs

	return cmd
}
