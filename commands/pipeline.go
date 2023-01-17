package commands

import (
	"fmt"
	"mycli/app"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// Pipeline is a shortcut to open an AWS pipeline to see its status or approve a pending one.
var Pipeline = &console.Command{
	Name:    "open:pipeline",
	Aliases: []*console.Alias{{Name: "pipeline"}},
	Usage:   "Open a specific pipeline on aws",
	Args: console.ArgDefinition{
		{Name: "name", Optional: false, Description: "The name of the pipeline to open"},
	},
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:     "prod",
			Aliases:  []string{"p"},
			Required: false,
			Usage:    "Open prod pipeline",
		},
	},
	Action: func(c *console.Context) error {
		name := c.Args().Get("name")

		pipelineAliases := app.GetMapConfig("PipelineAliases")

		alias, exists := pipelineAliases[name]
		if exists {
			name = alias
		}

		env := "staging"
		if c.Bool("prod") {
			env = "production"
		}

		suffixes := app.GetMapConfig("PipelineSuffixes")
		suffix := suffixes[name]

		pipelineUrlPattern := app.GetConfig("PipelineUrlTemplate")
		app.OpenCommand(fmt.Sprintf(pipelineUrlPattern, name, env, suffix))

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success(fmt.Sprintf("Opening pipeline %v-%v%v", name, env, suffix))

		return nil
	},
}
