package execer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/magdyamr542/reloader/config"
)

// Execer starts a program based on some configuration.
// It stops when the context is done.
type Execer interface {
	Exec(ctx context.Context, errors chan<- error) (Stopper, error)
}

// Stopper stops an execution of a program.
type Stopper func() error

// Execution represents the execution of a program.
type Execution struct {
	Command string
}

type execer struct {
	config config.Config
	logger *log.Logger
}

func New(config config.Config, logger *log.Logger) Execer {
	e := execer{config: config, logger: logger}
	return &e
}

func (r *execer) Exec(ctx context.Context, errorCh chan<- error) (Stopper, error) {

	config := r.config

	// Run the before command
	if config.Before != "" {
		r.logger.Printf("Running %q\n", config.Before)
		parts := strings.Split(config.Before, " ")
		cmd := newCmd(ctx, parts)
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("running command %q: %w", config.Before, err)
		}
	}

	// Run the command itself in a separate goroutine.
	r.logger.Printf("Running %q\n", config.Command)
	cmdContext, cancel := context.WithCancel(ctx)
	parts := strings.Split(config.Command, " ")
	cmd := newCmd(ctx, parts)
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("can't start command %q: %w", config.Command, err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			errorCh <- err
		}
	}()

	stopper := func() error {
		// Stop the current main program and wait for it.
		cancel()
		<-cmdContext.Done()

		// Run the after command if possible

		if config.After != "" {
			r.logger.Printf("Running %q\n", config.After)
			parts := strings.Split(config.After, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := newCmd(ctx, parts)
			if err := cmd.Run(); err != nil {
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
	cmd.Env = os.Environ()
	return cmd
}
