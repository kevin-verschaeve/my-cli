package commands

import (
	"fmt"
	"mycli/app"
	"strings"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// Linear allows to open a given ticket number in the browser.
//
// You can give the full ticket name, eg: "OPS-1234" or only "1234".
// In the second case, you can set the environment variable "LINEAR_TICKET_PREFIX" to the prefix of your team in linear.
//
// If no ticket number is provided, try to guess it from the current git branch name.
var Linear = &console.Command{
	Name:    "open:linear",
	Aliases: []*console.Alias{{Name: "linear"}, {Name: "lin"}},
	Usage:   "Open a specific linear ticket or the one corresponding to the current branch",
	Args: console.ArgDefinition{
		{Name: "ticket-id", Optional: true, Description: "The ticket id you want to see. If not provided, get the ticket id from the current git branch name."},
	},
	Action: func(c *console.Context) error {
		ticket := c.Args().Get("ticket-id")
		if ticket == "" {
			var err error
			ticket, err = app.RunGitCommand("branch", "--show")
			if err != nil {
				return err
			}
		} else {
			parts := strings.Split(ticket, "-")
			if len(parts) == 1 {
				prefix := app.GetConfig("LinearTicketPrefix")
				if prefix == "" {
					prefix = app.GetEnv("LINEAR_TICKET_PREFIX", "OPS")
				}

				ticket = fmt.Sprintf("%s-%s", prefix, parts[0])
			}
		}

		url := fmt.Sprintf("https://linear.app/%s/issue/%s", app.GetConfig("LinearOrganization"), strings.ToUpper(strings.TrimSuffix(ticket, "\n")))
		app.OpenCommand(url)

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success(fmt.Sprintf("Opening linear ticket : %s", url))

		return nil
	},
}
