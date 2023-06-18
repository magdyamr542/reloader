package runnable

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// Runnable is something that can run on the machine like a command.
type Runnable interface {
	// Run runs the executable. This starts and waits.
	Run() error
	// Start starts running the executable but doesn't wait for it.
	Start() error
	// Wait blocks till the executable is done.
	Wait() error
	// Signal sends a signal to the executable.
	Signal(signal os.Signal) error
}

// Creator is a function that returns a Runnable.
type Creator func(ctx context.Context, command []string) Runnable

type osCmd struct {
	cmd *exec.Cmd
}

func NewCmd(ctx context.Context, command []string) Runnable {
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
func (o *osCmd) Signal(signal os.Signal) error {
	if o.cmd.Process == nil {
		return nil
	}
	return o.cmd.Process.Signal(signal)
}

func newCmd(ctx context.Context, parts []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ()
	return cmd
}
