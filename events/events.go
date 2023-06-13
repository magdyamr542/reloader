package events

import "time"

// Watcher defines a set of files to be watched.
type Watcher struct {
	// The directory containing the files to be watched.
	Directory string
	// The file pattern. E.g '*.go' for all go files
	Pattern string
}

// Event is an event of a file change.
type Event struct {
	// This is the file's path.
	File string
	// The timestamp of the file being changed.
	Timestamp time.Time
}
