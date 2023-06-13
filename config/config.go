package config

// Config configures the reloader
type Config struct {
	// The command to execute before the main program
	Before string
	// The command to execute after the main program
	After string
	// The command to execute the main program
	Command string
	// The file patterns to watch
	Patterns []string
}
