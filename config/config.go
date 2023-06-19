package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	debugLogLevel = "DEBUG"
	infoLogLevel  = "INFO"
	errorLogLevel = "ERROR"
)

var logLevels = map[string]struct{}{
	debugLogLevel: {},
	infoLogLevel:  {},
	errorLogLevel: {},
}

// Config configures the reloader
type Config struct {
	// The command to execute before the main program
	Before []CommandWithDir `yaml:"before"`
	// The command to execute after the main program
	After []CommandWithDir `yaml:"after"`
	// The command to execute the main program
	Command CommandWithDir `yaml:"main"`
	// The file patterns to watch
	Patterns []string `yaml:"patterns"`
	// The log level to use
	LogLevel string `yaml:"loglevel,omitempty"`
}

// CommandWithDir defines a command to be executed inside some directory.
type CommandWithDir struct {
	Command string            `yaml:"command"`
	BaseDir string            `yaml:"directory,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

func ParseFromFile(path string) (Config, error) {
	var c Config

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal([]byte(fileBytes), &c)
	if err != nil {
		return c, err
	}

	// Clean the patterns.
	for i, p := range c.Patterns {
		c.Patterns[i] = strings.TrimSpace(p)
	}

	// Validate the commands
	if err := cleanCmd(&c.Command); err != nil {
		return c, fmt.Errorf("command: %v", err)
	}

	for _, cmd := range c.Before {
		if err := cleanCmd(&cmd); err != nil {
			return c, fmt.Errorf("before command: %v", err)
		}
	}

	for _, cmd := range c.After {
		if err := cleanCmd(&cmd); err != nil {
			return c, fmt.Errorf("after command: %v", err)
		}
	}

	// Validate log level.
	if c.LogLevel == "" {
		c.LogLevel = debugLogLevel
	} else if _, ok := logLevels[c.LogLevel]; !ok {
		return c, fmt.Errorf("")
	}

	return c, nil
}

func cleanCmd(cmd *CommandWithDir) error {
	cmd.Command = strings.TrimSpace(cmd.Command)
	if cmd.Command == "" {
		return fmt.Errorf("command is empty")
	}

	if cmd.BaseDir == "" {
		cwd, err := filepath.Abs("./")
		if err != nil {
			return fmt.Errorf("can't resolve cwd: %v", err)
		}
		cmd.BaseDir = cwd
	}

	fileinfo, err := os.Stat(cmd.BaseDir)
	if err != nil {
		return fmt.Errorf("can't stat directory %s: %v", cmd.BaseDir, err)
	}

	if !fileinfo.IsDir() {
		return fmt.Errorf("%s isn't a directory", cmd.BaseDir)
	}

	return nil
}
