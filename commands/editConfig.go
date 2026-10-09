package commands

import (
	"fmt"
	"log"
	"mycli/app"
	"os"
	"os/exec"

	"github.com/kballard/go-shellquote"
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
		editor := os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "nano"
		}

		args, err := shellquote.Split(editor)
		if err != nil || len(args) == 0 {
			return fmt.Errorf("invalid editor command: %q", editor)
		}
		cmd := exec.Command(args[0], append(args[1:], app.MyCliHome()+"/"+app.CONFIG_FILE)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	},
}
