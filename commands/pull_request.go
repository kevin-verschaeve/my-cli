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
		vcs := app.GetConfig("VersionControlService")

		if vcs == "github" {
			cmd := exec.Command("gh", "pr", "view", "--web")
			cmd.Run()

			ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
			ui.Success("Opening pull request on github")

			return nil
		}

		if vcs == "gitlab" {
			cmd := exec.Command("lab", "mr", "browse")
			cmd.Run()

			ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
			ui.Success("Opening pull request on gitlab")

			return nil
		}

		log.Fatal("vcs not correctly set or supported (only github and gitlab are supported)")

		return nil
	},
}
