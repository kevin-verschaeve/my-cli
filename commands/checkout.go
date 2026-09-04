package commands

import (
	"fmt"
	"mycli/app"
	"strings"

	"github.com/symfony-cli/console"
)

// Checkout the current branch using the pre defined prefix
var Checkout = &console.Command{
	Name:    "checkout",
	Aliases: []*console.Alias{{Name: "go"}},
	Usage:   "Checkout the current branch using the pre defined prefix",
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:         "create-branch",
			Aliases:      []string{"b"},
			Required:     false,
			DefaultValue: false,
			Usage:        "Try to automatically create the branch from develop if it doesn't already exists.",
		},
	},
	Args: console.ArgDefinition{
		{Name: "ticket-id", Optional: false, Description: "The ticket id you want to checkout."},
	},
	Action: func(c *console.Context) error {
		prefix := app.GetConfig("LinearTicketPrefix")
		if prefix == "" {
			prefix = app.GetEnv("LINEAR_TICKET_PREFIX", "OPS")
		}

		prefix = strings.ToLower(prefix)
		branch := fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(strings.ToLower(c.Args().Get("ticket-id")), fmt.Sprintf("%s-", prefix), ""))
		out, err := app.RunGitCommand("checkout", branch)

		if err != nil {
			if c.Bool("create-branch") {
				res, e := app.RunGitCommand("checkout", "-b", branch, "origin/develop")

				if e == nil {
					fmt.Print(res)

					return nil
				}

				return err
			}
		}

		fmt.Print(out)

		return nil
	},
}
