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

// OpenPullRequest allows to open the Pull Request in github corresponding to the current branch.
var OpenPullRequest = &console.Command{
	Name:    "open:pr",
	Aliases: []*console.Alias{{Name: "pr"}},
	Usage:   "Open the current branch Pull Request",
	Args: console.ArgDefinition{
		{Name: "ticket-id", Optional: true, Description: "The ticket id to check out the pull request. If None, guess it from the current branch"},
	},
	Before: func(c *console.Context) error {
		vcs := app.GetConfig("VersionControlService")

		if vcs == "github" {
			if err := app.CheckCommandExists("gh"); err != nil {
				log.Fatal("Github cli (gh) is required to run this command. Install it from here: https://github.com/cli/cli#installation")
			}
		}

		if vcs == "gitlab" {
			if err := app.CheckCommandExists("lab"); err != nil {
				log.Fatal("Gitlab cli (lab) is required to run this command. Install it from here: https://gitlab.com/lab-cli/lab#installation")
			}
		}

		return nil
	},
	Action: func(c *console.Context) error {
		ticketId := c.Args().Get("ticket-id")
		vcs := app.GetConfig("VersionControlService")

		prefix := app.GetConfig("LinearTicketPrefix")
		if prefix == "" {
			prefix = app.GetEnv("LINEAR_TICKET_PREFIX", "OPS")
		}
		prefix = strings.ToLower(prefix)
		branch := fmt.Sprintf("%s-%s", prefix, strings.ReplaceAll(strings.ToLower(ticketId), fmt.Sprintf("%s-", prefix), ""))

		if vcs == "github" {
			var cmd *exec.Cmd
			if ticketId == "" {
				cmd = exec.Command("gh", "pr", "view", "--web")
			} else {
				cmd = exec.Command("gh", "pr", "view", branch, "--web")
			}

			cmd.Run()

			ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
			ui.Success("Opening pull request on github")

			return nil
		}

		if vcs == "gitlab" {
			var cmd *exec.Cmd
			if ticketId == "" {
				cmd = exec.Command("lab", "mr", "browse")
			} else {
				cmd = exec.Command("lab", "mr", "browse", branch)
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

		log.Fatal("vcs not correctly set or supported (only github and gitlab are supported)")

		return nil
	},
}
