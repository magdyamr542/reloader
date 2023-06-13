package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/magdyamr542/reloader/config"
	"github.com/magdyamr542/reloader/events"
	"github.com/magdyamr542/reloader/execer"
	"github.com/magdyamr542/reloader/notifier"
)

func main() {
	logger := log.New(os.Stdout, "[reloader] ", log.Ldate|log.Ltime)

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
		logger.Fatalf("command is required.")
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

	// Build the watchers. These are all possible patterns:
	// server/*.go
	// server/client/*.go
	// server.go
	// *.go
	// /abs/path/to/dir/*.go
	watchers := make([]events.Watcher, 0)
	for _, p := range c.Patterns {
		dir, pattern := filepath.Split(p)

		if !filepath.IsAbs(dir) {
			absDir, err := filepath.Abs(dir)
			if err != nil {
				logger.Fatalf("Can't get the absolute path from pattern %s: %v", p, err)
			}
			dir = absDir
		}

		logger.Printf("Will watch %s\n", filepath.Join(dir, pattern))
		watchers = append(watchers, events.Watcher{
			Directory: dir,
			Pattern:   pattern,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop on interruption.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		logger.Printf("Got signal to stop. Cancelling the context\n")
		cancel()
	}()

	// Events and Errors
	eventCh := make(chan events.Event)
	errWatchFilesCh := make(chan error)

	// Notifier
	notifier := notifier.New(logger)
	watcherCloser, err := notifier.Notify(ctx, watchers, eventCh, errWatchFilesCh)
	defer watcherCloser()
	if err != nil {
		log.Fatalf("init notifier: %v", err)
	}

	// Execer
	// Start for the first time and expect no errors.
	errMainProgramCh := make(chan error)
	exc := execer.New(c, logger)
	stopper, err := exc.Exec(ctx, errMainProgramCh)
	if err != nil {
		log.Fatalf("execute program: %v", err)
	}

	// Watch for file changes and re execute the program
	go func(stopper execer.Stopper) {
		defer cancel()
		logger.Printf("Starting the watch loop\n")

		for {
			select {
			case err := <-errWatchFilesCh:
				logger.Printf("Error watch files: %v", err)

			case err := <-errMainProgramCh:
				logger.Printf("Error running the main program. Stopping the watch loop: %v", err)
				return

			case <-ctx.Done():
				logger.Printf("Context done. Stopping the watch loop\n")
				return

			case event := <-eventCh:
				logger.Printf("File %s changed at %v. Stopping the current execution...", event.File, event.Timestamp.Format("01-02-2006 15:04:05"))

				// First stop the current execution. This will stop the current main program and then execute
				// the will run the 'after' command if it exists.
				err := stopper()
				if err != nil {
					logger.Printf("Error while stopping the current execution: %v", err)
					return
				}

				logger.Printf("Stopped the current execution. Reloading...\n")

				// Then rerun the program again.
				stopper, err = exc.Exec(ctx, errMainProgramCh)
				if err != nil {
					logger.Fatalf("execute program: %v", err)
					return
				}

			}
		}

	}(stopper)

	<-ctx.Done()
}
