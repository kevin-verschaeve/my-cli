package commands

import (
	"bufio"
	"fmt"
	"mycli/app"
	"os"
	"strings"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// MultiCherryPicker allows to apply the commits of the current branch to a given branch and then push them.
//
// Here is a summary of what this command does:
// - Check all the commits of the current branch compared to master
// - Shows the commit to the user and ask confirmation before continue
// - Go to the branch and update it using rebase
// - Cherry pick the commits
// - If cherry-pick gone well, push to the branch
// - If cherry-pick gone bad, stops the process and you have to fix conflicts and finish manually
var MultiCherryPicker = &console.Command{
	Name:    "cherry-pick",
	Aliases: []*console.Alias{{Name: "pick"}},
	Usage:   "Get commits from current branch and apply them to a given branch",
	Args: console.ArgDefinition{
		{Name: "branch", Optional: true, Description: "The branch name to cherry-pick to", Default: "staging"},
	},
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:     "pick-only",
			Aliases:  []string{"no-push", "n"},
			Required: false,
			Usage:    "If present, the commits will not be pushed to the branch, but only cherry-picked",
		},
	},
	Action: func(c *console.Context) error {
		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)

		// get commit list to push
		commits, err := app.RunGitCommand("log", "--format=\"%H\"", "origin/master..HEAD")
		if err != nil {
			return err
		}

		commitsWithMessage, err := app.RunGitCommand("log", "--format=\"%H %s\"", "origin/master..HEAD")
		if err != nil {
			return err
		}

		nb := len(strings.Split(commits, "\n")) - 1

		buf := bufio.NewReader(os.Stdin)
		fmt.Println("Cherry-pick", nb, "commit(s) ? [Y/n]")
		fmt.Println(commitsWithMessage)
		fmt.Print("> ")

		answer, err := buf.ReadString('\n')
		answer = strings.TrimSuffix(answer, "\n")

		if len(answer) == 0 {
			answer = "y"
		}

		if err != nil {
			return err
		}

		if strings.ToLower(answer) != "y" {
			ui.Warning("Aborted")

			return nil
		}

		branch := c.Args().Get("branch")

		// checkout branch
		ui.Note(fmt.Sprintf("Checkout %s...", branch))
		app.RunGitCommand("checkout", branch)

		var info string

		// update branch
		ui.Note(fmt.Sprintf("Updating %s...", branch))
		if info, err = app.RunGitCommand("pull", "--rebase", "origin", branch); err != nil {
			return err
		}
		ui.Comment(info)

		// cherry pick the commits
		ui.Note("Cherry-picking the commits...")
		cherryPick := append([]string{"cherry-pick"}, strings.Split(strings.Replace(strings.TrimSpace(commits), "\"", "", -1), "\n")...)
		if info, err = app.RunGitCommand(cherryPick...); err != nil {
			fmt.Println("Cherry pick has failed, there were probably some conflicts")
			return err
		}
		ui.Comment(info)

		if c.Bool("pick-only") {
			return nil
		}

		// push to branch
		ui.Note(fmt.Sprintf("Pushing to %s...", branch))
		if info, err = app.RunGitCommand("push", "origin", branch); err != nil {
			return err
		}
		ui.Comment(info)

		ui.Success(fmt.Sprintf("%s branch successfully updated", branch))

		return nil
	},
}
