package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/magdyamr542/reloader/config"
	"github.com/magdyamr542/reloader/events"
	"github.com/magdyamr542/reloader/execer"
	"github.com/magdyamr542/reloader/notifier"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

var loglevels = []string{"trace", "debug", "error", "info", "warn", "error"}

func run() error {

	// Flags
	loglevel := flag.String("loglevel", "debug", fmt.Sprintf("The log level. One of %v. If provided wrong, logs will be disabled.", loglevels))
	before := flag.String("before", "", "The command to execute before running the main program.")
	after := flag.String("after", "", "The command to execute after running the main program.")
	command := flag.String("cmd", "", "The command to execute the main program. (required)")
	patterns := flag.String("patterns", "*", "Unix like file patters to watch for changes.\n"+
		"This is a space separated list. E.g: 'src/cmd/*.go src/server/*.go'.\n"+
		"The program will reload after a file of those changes.")

	flag.Parse()

	// Build the config
	cmd := strings.TrimSpace(*command)
	if cmd == "" {
		flag.Usage()
		return fmt.Errorf("command is required")
	}

	realPatterns := []string{}
	for _, p := range strings.Split(*patterns, " ") {
		trimmed := strings.TrimSpace(p)
		if len(trimmed) != 0 {
			realPatterns = append(realPatterns, trimmed)
		}
	}

	c := config.Config{
		Before:   *before,
		After:    *after,
		Command:  cmd,
		Patterns: realPatterns,
	}

	// Build the logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "reloader",
		Level: hclog.LevelFromString(*loglevel),
	})

	// Build the watchers. These are all possible patterns:
	// server/*.go
	// server/client/*.go
	// server.go
	// *.go
	// /abs/path/to/dir/*.go
	logger.Debug("Building the file watchers...")
	watchers := make([]events.Watcher, 0)
	for _, p := range c.Patterns {
		dir, pattern := filepath.Split(p)

		if !filepath.IsAbs(dir) {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("can't get the absolute path from pattern %s: %v", p, err)
			}
			dir = absDir
		}

		logger.Debug("Will watch", "path", filepath.Join(dir, pattern))
		watchers = append(watchers, events.Watcher{
			Directory: dir,
			Pattern:   pattern,
		})
	}

	topLevelCtx, topLevelCancel := context.WithCancel(context.Background())
	defer topLevelCancel()

	// Stop on interruption.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		topLevelCancel()
	}()

	// Events and Errors
	eventCh := make(chan events.Event)
	errWatchFilesCh := make(chan error)

	// Notifier
	logger.Debug("Init the notifier...")
	notifier := notifier.New(logger.Named("notifier"))
	watcherCloser, err := notifier.Notify(topLevelCtx, watchers, eventCh, errWatchFilesCh)
	defer watcherCloser()
	if err != nil {
		return fmt.Errorf("init notifier: %v", err)
	}

	// Execer
	// Start for the first time and expect no errors.
	exc := execer.New(c, logger.Named("execer"))
	stopper, err := exc.Exec(topLevelCtx)
	if err != nil {
		return fmt.Errorf("execute program: %v", err)
	}

	// Watch for file changes and re execute the program

	logger.Debug("Starting watch loop...")
	watchLoopDone := make(chan struct{}, 1)
	var errWatchLoop error
	go func(stopper execer.Stopper, outerCtx context.Context) {
		defer topLevelCancel()
		defer func() { watchLoopDone <- struct{}{} }()

		for {
			select {
			case err := <-errWatchFilesCh:
				logger.Error("Watch files", "err", err)

			case <-outerCtx.Done():
				logger.Info("Stopping the current process and exiting...")
				err := stopper()
				if err != nil {
					logger.Error("Stopping the current process", "err", err)
					errWatchLoop = err
				}
				return

			case event := <-eventCh:
				logger.Debug("File changed. Stopping the current process...", "file", event.File, "changedAt", event.Timestamp.Format("01-02-2006 15:04:05"))

				// First stop the current execution. This will stop the current main program and then execute
				// the will run the 'after' command if it exists.
				err := stopper()
				if err != nil {
					logger.Error("Stopping the current process", "err", err)
					errWatchLoop = err
					return
				}

				logger.Debug("Stopped the current process. Rerun...")

				// Then rerun the program again.
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stopper, err = exc.Exec(ctx)
				if err != nil {
					logger.Error("Executing program", "err", err)
					errWatchLoop = err
					return
				}

			}
		}

	}(stopper, topLevelCtx)

	<-topLevelCtx.Done()
	<-watchLoopDone

	return errWatchLoop
}
