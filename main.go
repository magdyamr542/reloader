package main

import (
	"flag"
	"log"
	"os"
	"strings"
)

type Config struct {
	Before   string
	After    string
	Command  string
	Patterns []string
}

func main() {
	logger := log.New(os.Stdout, "[reloader] ", log.Ldate|log.Ltime)
	before := flag.String("before", "", "The command to execute before running the main program.")
	after := flag.String("after", "", "The command to execute after running the main program.")
	command := flag.String("cmd", "", "The command to execute the main program. (required)")
	patterns := flag.String("patterns", "*", "Unix like file patters to watch for changes.\n"+
		"This is a space separated list. E.g: 'src/cmd/*.go src/server/*.go'.\n"+
		"The program will reload after a file of those changes.")

	flag.Parse()

	cmd := strings.TrimSpace(*command)
	if cmd == "" {
		flag.Usage()
		logger.Fatalf("command is required.")
	}

	c := Config{
		Before:   *before,
		After:    *after,
		Command:  cmd,
		Patterns: strings.Split(strings.TrimSpace(*patterns), " "),
	}

	logger.Printf("config %+v\n", c)

}
