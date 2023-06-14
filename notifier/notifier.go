package notifier

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/magdyamr542/reloader/events"
)

// A notifier takes a set of watchers to watch and emits events when a file of those being watched changes.
// Errors will be sent over the error channel.
// The watcher keeps watching files until the ctx is done.
// The watcher returns a closer to stop watching the files.
type Notifier interface {
	Notify(ctx context.Context, watchers []events.Watcher, events chan<- events.Event, errors chan<- error) (Closer, error)
}

type Closer func() error

type notifier struct {
	logger *log.Logger
}

func New(logger *log.Logger) Notifier {
	return &notifier{logger: logger}
}

func (n *notifier) Notify(ctx context.Context,
	watchers []events.Watcher,
	eventCh chan<- events.Event,
	errorCh chan<- error) (Closer, error) {

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					// Check if the file satisfies the watcher's patterns.
					match := isMatch(watchers, event.Name)
					if match {
						eventCh <- events.Event{File: event.Name, Timestamp: time.Now()}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				errorCh <- err
			case <-ctx.Done():
				n.logger.Printf("Stopping the files watcher...\n")
				return
			}
		}
	}()

	// Watch paths
	for _, w := range watchers {
		err = watcher.Add(w.Directory)
		if err != nil {
			return nil, fmt.Errorf("watch %s: %w", w.Directory, err)
		}
	}

	return func() error {
		return watcher.Close()
	}, nil
}

func isMatch(watchers []events.Watcher, fileAbsPath string) bool {
	for _, w := range watchers {
		absPattern := filepath.Join(w.Directory, w.Pattern)
		if ok, err := filepath.Match(absPattern, fileAbsPath); err == nil && ok {
			return true
		}
	}
	return false
}
