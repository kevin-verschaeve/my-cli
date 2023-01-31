package commands

import (
	"mycli/app"

	"github.com/symfony-cli/console"
)

// ConfigEdit open a file editor to see or edit the configuration of this cli
var ConfigEdit = &console.Command{
	Name:    "config:edit",
	Aliases: []*console.Alias{{Name: "config"}},
	Usage:   "Show or edit configuration",
	Action: func(c *console.Context) error {
		app.OpenCommand(app.MyCliHome() + "/" + app.CONFIG_FILE)

		return nil
	},
}
