package execer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/magdyamr542/reloader/config"
	"github.com/magdyamr542/reloader/runnable"
)

var (
	// durationWaitBeforeHardKill is the duration to wait after sending the process an Interrupt signal. If the process,
	// doesn't exit, we send a Kill signal.
	durationWaitBeforeHardKill = 10 * time.Second
)

// Execer starts a program based on some configuration.
// It stops when the context is done.
type Execer interface {
	Exec(ctx context.Context) (Stopper, <-chan error, error)
}

// Stopper stops an execution of a program.
type Stopper func() error

// Execution represents the execution of a program.
type Execution struct {
	Command string
}

type execer struct {
	config          config.Config
	logger          hclog.Logger
	runnableCreator runnable.Creator
}

func New(config config.Config, logger hclog.Logger, runnableCreator runnable.Creator) Execer {
	e := execer{config: config, logger: logger, runnableCreator: runnableCreator}
	return &e
}

func (r *execer) Exec(ctx context.Context) (Stopper, <-chan error, error) {

	config := r.config

	// Run the before commands
	for _, command := range config.Before {
		r.logger.Info("Running before command", "command", command.Command)
		beforeCmdCtx, beforeCmdCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer beforeCmdCancel()
		beforeCmd := r.runnableCreator(beforeCmdCtx, command)
		if err := beforeCmd.Run(); err != nil {
			return nil, nil, fmt.Errorf("running before command %q: %w", command.Command, err)
		}
	}

	// Run the command itself in a separate goroutine.
	r.logger.Info("Running command", "command", config.Command.Command)
	mainCmdCtx, mainCmdCancel := context.WithCancel(ctx)
	mainCmd := r.runnableCreator(mainCmdCtx, config.Command)
	cmdErrCh, err := mainCmd.RunCh()
	if err != nil {
		mainCmdCancel()
		return nil, nil, fmt.Errorf("can't start command %q: %w", config.Command, err)
	}

	errCh := make(chan error, 1)
	killed := false
	interrupted := false
	wasStopped := false
	go func() {
		err := <-cmdErrCh
		if !wasStopped {
			errCh <- err
		}
	}()

	stopper := func() error {
		wasStopped = true
		r.logger.Debug("Stopping by sending an Interrupt...")

		// Cancel the ctx, thus ending the cmd. If the sent SigTerm doesn't respond within 10 seconds, hard kill.
		mainCmdCancel()
		go func() {
			time.AfterFunc(durationWaitBeforeHardKill, func() {
				if !interrupted {
					r.logger.Debug("Program didn't stop by Interrupt. Hard killing...", "durationPassed", durationWaitBeforeHardKill)
					if err := mainCmd.Kill(); err != nil {
						r.logger.Error("Error hard killing the program", "err", err)
					}
					r.logger.Debug("Program was hard killed")
					killed = true
				}
			})
		}()

		r.logger.Debug("Waiting for program to be done...")
		err := <-cmdErrCh
		if !killed {
			interrupted = true
		}
		r.logger.Debug("Program is done", "killed", killed, "interrupted", interrupted, "err", err)
		if err != nil {
			exitErr, isExit := err.(*exec.ExitError)
			if !isExit {
				return err
			}

			// Check the underlying process state's exit status
			status, isWait := exitErr.Sys().(syscall.WaitStatus)
			if !isWait {
				return err
			}

			// Ignore the error if the process was killed by a signal
			if status.Signaled() && (status.Signal() == os.Interrupt || status.Signal() == os.Kill || status.Signal() == syscall.SIGTERM) {
				r.logger.Debug("Process exited by signal", "signal", status.Signal(),
					"pid", exitErr.ProcessState.Pid, "exitCode", exitErr.ProcessState.ExitCode())
			} else {
				return err
			}
		}

		// Run the after command if possible
		for _, command := range config.After {
			r.logger.Info("Running after command", "command", command.Command)
			afterCmdCtx, afterCmdCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer afterCmdCancel()
			afterCmd := r.runnableCreator(afterCmdCtx, command)
			if err := afterCmd.Run(); err != nil {
				return fmt.Errorf("running command %q: %w", command.Command, err)
			}
		}

		return nil
	}

	return stopper, errCh, nil
}
