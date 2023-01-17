// Package main contains the entrypoint of the tool.
package main

import (
	"mycli/commands"
	"os"

	"github.com/symfony-cli/console"
)

func main() {
	app := &console.Application{
		Name:     "My CLI",
		Usage:    "My CLI provide some commands for everyday usage",
		Commands: commands.CommonCommands(),
		Version:  "1.0.0",
	}
	app.Run(os.Args)
}
