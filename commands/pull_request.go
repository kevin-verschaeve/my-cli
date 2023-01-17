package commands

import (
	"log"
	"mycli/app"
	"os/exec"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// OpenPullRequest allows to open the Pull Request in github corresponding to the current branch.
var OpenPullRequest = &console.Command{
	Name:    "open:pr",
	Aliases: []*console.Alias{{Name: "pr"}},
	Usage:   "Open the current branch Pull Request",
	Before: func(c *console.Context) error {
		if err := app.CheckCommandExists("gh"); err != nil {
			log.Fatal("github cli is required to run this command. Install it from here: https://github.com/cli/cli#installation")
		}

		return nil
	},
	Action: func(c *console.Context) error {
		cmd := exec.Command("gh", "pr", "view", "--web")
		cmd.Run()

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success("Opening pull request on github")

		return nil
	},
}
