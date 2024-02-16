package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"mycli/app"
	"os/exec"
	"strconv"
	"strings"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// PullRequest represents the output of the command "gh pr view --json".
type PullRequest struct {
	Number int
}

// Preview is a command that opens a specific amplify environment preview for a pull request.
var Preview = &console.Command{
	// hide the command as we don't have preview apps right now
	Hidden: func() bool {
		return true
	},
	Name:    "open:preview",
	Aliases: []*console.Alias{{Name: "preview"}},
	Usage:   "Open a specific environment preview for a pull request",
	Args: console.ArgDefinition{
		{Name: "pr-number", Optional: true, Description: "The Pull Request number to preview or nothing to guess it from the current branch"},
	},
	Action: func(c *console.Context) error {
		number, err := strconv.Atoi(c.Args().Get("pr-number"))

		if err != nil {
			if err = app.CheckCommandExists("gh"); err != nil {
				log.Fatal("github cli is required to run this command. Install it from here: https://github.com/cli/cli#installation")
			}

			cmd, out := exec.Command("gh", "pr", "view", "--json", "number"), new(strings.Builder)
			cmd.Stdout = out
			cmd.Run()

			var result PullRequest
			json.Unmarshal([]byte(out.String()), &result)

			number = result.Number
		}

		template := app.GetConfig("PreviewUrlTemplate")
		url := fmt.Sprintf(template, number)
		app.OpenCommand(url)

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success(fmt.Sprintf("Opening preview environement for pull request n° %d : %v", number, url))

		return nil
	},
}
