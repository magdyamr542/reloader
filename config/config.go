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
	Before []CommandWithDir `yaml:"runBefore"`
	// The command to execute after the main program
	After []CommandWithDir `yaml:"runAfter"`
	// The command to execute the main program
	Command CommandWithDir `yaml:"run"`
	// The file patterns to watch
	Patterns []string `yaml:"filePatterns"`
	// The log level to use
	LogLevel string `yaml:"loglevel,omitempty"`
}

// CommandWithDir defines a command to be executed inside some directory.
type CommandWithDir struct {
	Command Command           `yaml:"command"`
	BaseDir string            `yaml:"directory,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type Command struct {
	Program string   `yaml:"program"`
	Args    []string `yaml:"args"`
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
	cmd.Command.Program = strings.TrimSpace(cmd.Command.Program)
	if cmd.Command.Program == "" {
		return fmt.Errorf("program is empty")
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
