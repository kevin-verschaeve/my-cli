// Package commands contains every commands.
// Commands are defined as variables and must be registered in list.go file.
package commands

import "github.com/symfony-cli/console"

// CommonCommands registers all the available commands in the application.
func CommonCommands() []*console.Command {
	commands := []*console.Command{
		Daily,
		Linear,
		OpenProject,
		ConfigEdit,
		Checkout,
		Token,
		Hotfix,
	}

	return commands
}
