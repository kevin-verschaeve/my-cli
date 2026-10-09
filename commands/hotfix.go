package commands

import (
	"mycli/hotfix"

	"github.com/symfony-cli/console"
)

var Hotfix = &console.Command{
	Name:  "hotfix",
	Usage: "Hotfix workflow: start, publish, finish, resume, or status; omit the action for an interactive menu",
	Args: console.ArgDefinition{
		{Name: "action", Optional: true, Description: "start, publish, finish, resume, or status"},
	},
	Action: hotfix.Run,
}
