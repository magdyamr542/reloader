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
	// RunCh runs the executable but doesn't block. It returns a chan of err that receives the error returned
	// by the executable after it's finished running.
	RunCh() (<-chan error, error)
	// Kill hard kills the executable (and any children created).
	Kill() error
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

func (o *osCmd) RunCh() (<-chan error, error) {
	if err := o.cmd.Start(); err != nil {
		return nil, err
	}
	errCh := make(chan error, 2)
	go func() {
		err := o.cmd.Wait()
		// We are notifying at most 2 things here.
		// 1. The routine that waits for an error from this process and propagates upwards. This is done when the main
		// command exists without the being stopped (without someone calling the stopper()).
		// 2. The stopper() itself of the error, in this case a file was changed and the stopper() was called,
		// the routine described in 1 should ignore this error since it's not really a program error but an error we get
		// because we explicitly want to restart the command.
		errCh <- err
		errCh <- err

	}()
	return errCh, nil
}

func (o *osCmd) Kill() error {
	return syscall.Kill(-o.cmd.Process.Pid, syscall.SIGKILL)
}

func newCmd(ctx context.Context, command config.CommandWithDir) *exec.Cmd {
	parts := strings.Split(command.Command, " ")
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	// Setting this makes newly created processes have the same pgid. So, if the main command is a bash script that
	// creates 2 processes (so 3 processes in total), all these processes get the same pgid. Now, when trying to kill
	// the process created by the main command (the bash process), we also want to kill the 2 processes created by it.
	// This can be achieved since all 3 processes share the same pgid. We send a signal using -"pgid" which signals
	// all processes that have group id of pgid. All child processed get pgid as pid of the parent process.
	// So we send a signal to -"parent.pid" which is equivalent to -"pgid" of the created child processes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = command.BaseDir
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
	}

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
