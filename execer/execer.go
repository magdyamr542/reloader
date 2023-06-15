package execer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/magdyamr542/reloader/config"
)

// Execer starts a program based on some configuration.
// It stops when the context is done.
type Execer interface {
	Exec(ctx context.Context) (Stopper, error)
}

// Stopper stops an execution of a program.
type Stopper func() error

// Execution represents the execution of a program.
type Execution struct {
	Command string
}

type execer struct {
	config config.Config
	logger hclog.Logger
}

func New(config config.Config, logger hclog.Logger) Execer {
	e := execer{config: config, logger: logger}
	return &e
}

func (r *execer) Exec(ctx context.Context) (Stopper, error) {

	config := r.config

	// Run the before command
	if config.Before != "" {
		r.logger.Info("Running before command", "command", config.Before)
		parts := strings.Split(config.Before, " ")
		beforeCmd := newCmd(ctx, parts)
		if err := beforeCmd.Run(); err != nil {
			return nil, fmt.Errorf("running command %q: %w", config.Before, err)
		}
	}

	// Run the command itself in a separate goroutine.
	r.logger.Info("Running command", "command", config.Command)
	cmdContext, cancel := context.WithCancel(ctx)
	parts := strings.Split(config.Command, " ")
	mainCmd := newCmd(cmdContext, parts)
	err := mainCmd.Start()
	if err != nil {
		return nil, fmt.Errorf("can't start command %q: %w", config.Command, err)
	}

	mainCmdDone := make(chan struct{}, 1)
	go func() {
		mainCmd.Wait()
		mainCmdDone <- struct{}{}
	}()

	stopper := func() error {
		// Stop the current main program and wait for it.
		cancel()
		<-cmdContext.Done()

		if mainCmd.Process != nil {
			err := mainCmd.Process.Signal(os.Interrupt)
			if err != nil {
				return err
			}
		}

		// Wait for the main cmd to finish.
		<-mainCmdDone

		// Run the after command if possible
		if config.After != "" {
			r.logger.Info("Running after command", "command", config.After)
			parts := strings.Split(config.After, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			afterCmd := newCmd(ctx, parts)
			if err := afterCmd.Run(); err != nil {
				return fmt.Errorf("running command %q: %w", config.After, err)
			}
		}

		return nil
	}

	return stopper, nil
}

func newCmd(ctx context.Context, parts []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ()
	return cmd
}
