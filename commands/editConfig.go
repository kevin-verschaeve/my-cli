package commands

import (
	"log"
	"mycli/app"
	"os"

	"github.com/symfony-cli/console"
)

// ConfigEdit open a file editor to see or edit the configuration of this cli
var ConfigEdit = &console.Command{
	Name:    "config:edit",
	Aliases: []*console.Alias{{Name: "config"}},
	Usage:   "Show or edit configuration",
	Before: func(c *console.Context) error {
		if _, err := os.Stat(app.MyCliHome() + "/" + app.CONFIG_FILE); os.IsNotExist(err) {
			if err := os.MkdirAll(app.MyCliHome(), os.ModePerm); err != nil {
				log.Fatal("Unable to create home directory. Try to create it manually")
			}

			d1 := []byte("{\n\n}")
			err := os.WriteFile(app.MyCliHome()+"/"+app.CONFIG_FILE, d1, 0644)
			if err != nil {
				log.Fatal(err)
			}
		}

		return nil
	},
	Action: func(c *console.Context) error {
		app.OpenCommand(app.MyCliHome() + "/" + app.CONFIG_FILE)

		return nil
	},
}
