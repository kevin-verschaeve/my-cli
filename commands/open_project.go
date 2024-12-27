package commands

import (
	"fmt"
	"mycli/app"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// OpenProject allows to open a project in a specific environment the browser.
var OpenProject = &console.Command{
	Name:    "open",
	Aliases: []*console.Alias{{Name: "o"}},
	Usage:   "Open a specific project using shortcuts",
	Args: console.ArgDefinition{
		{Name: "project", Description: "The project to open in the browser."},
		{Name: "env", Optional: true, Description: "The environment to open the app in. Defaults to local", Default: "local"},
	},
	Action: func(c *console.Context) error {
		project := c.Args().Get("project")
		env := c.Args().Get("env")

		projectAlias := map[string]string{
			"pq":     "prometheusv2",
			"promv2": "prometheusv2",
			"prom":   "prometheus",
			"qq":     "quick-quote",
			"uid":    "unique-id",
		}

		projectName, projectAliasExists := projectAlias[project]

		if !projectAliasExists {
			projectName = project
		}

		envAlias := map[string]string{
			"int":  "eksin.aws",
			"va":   "vaapps",
			"pr":   "apps",
			"prod": "apps",
		}

		var projectEnv, tld string

		if env == "local" {
			projectEnv = projectName
			tld = env
		} else {
			envName, envAliasExists := envAlias[env]
			if !envAliasExists {
				envName = env
			}

			projectEnv = fmt.Sprintf("%s.%s", projectName, envName)
			tld = "com"
		}

		url := fmt.Sprintf("https://%s.exotec.%s", projectEnv, tld)
		app.OpenCommand(url)

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success(fmt.Sprintf("Opening project: %s", url))

		return nil
	},
}
