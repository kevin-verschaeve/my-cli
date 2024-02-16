package commands

import (
	"fmt"
	"log"
	"mycli/app"
	"os/exec"
	"strings"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// Pipeline is a shortcut to open an AWS pipeline to see its status or approve a pending one.
var Pipeline = &console.Command{
	Name:    "open:pipeline",
	Aliases: []*console.Alias{{Name: "pipeline"}, {Name: "ci"}},
	Usage:   "View a pipeline in the terminal or in the web",
	Args: console.ArgDefinition{
		{Name: "ticket-id", Optional: true, Description: "A ticket id to get the last pipeline from"},
	},
	Action: func(c *console.Context) error {
		vcs := app.GetConfig("VersionControlService")
		ticketId := c.Args().Get("ticket-id")

		prefix := app.GetConfig("LinearTicketPrefix")
		if prefix == "" {
			prefix = app.GetEnv("LINEAR_TICKET_PREFIX", "OPS")
		}
		prefix = strings.ToLower(prefix)
		branch := fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(strings.ToLower(ticketId), fmt.Sprintf("%s-", prefix), ""))

		if vcs == "gitlab" {
			var cmd *exec.Cmd
			if ticketId == "" {
				cmd = exec.Command("lab", "ci", "view")
			} else {
				cmd = exec.Command("lab", "ci", "view", branch)
			}

			err := cmd.Run()

			ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
			if err != nil {
				ui.Error("MR not found or merged.")
				return nil
			}

			ui.Success("Opening merge request on gitlab")

			return nil
		}

		log.Fatal("vcs not correctly set or supported (only gitlab is supported for this command)")

		return nil
	},
}
