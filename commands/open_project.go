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
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:         "docs",
			Aliases:      []string{"doc"},
			Required:     false,
			DefaultValue: false,
			Usage:        "Open the API documentation of the project instead of the app.",
		},
	},
	Args: console.ArgDefinition{
		{Name: "project", Optional: true, Description: "The project to open in the browser."},
		{Name: "env", Optional: true, Description: "The environment to open the app in. Defaults to local", Default: "local"},
	},
	Action: func(c *console.Context) error {
		project := c.Args().Get("project")
		env := c.Args().Get("env")

		if project == "" {
			project = app.GetProjectDir()
		}

		projectAlias := map[string]string{
			"pq":    "pily-quotation",
			"prom":  "prometheus",
			"qq":    "quick-quote",
			"uid":   "unique-id",
			"qqbff": "quick-quote",
		}

		projectName, projectAliasExists := projectAlias[project]

		if !projectAliasExists {
			projectName = project
		}

		if env == "qa" {
			projectName = fmt.Sprintf("%s-qa", projectName)
		}

		docs := c.Bool("docs")
		if docs {
			projectName = fmt.Sprintf("%s-api", projectName)
		}

		envAlias := map[string]string{
			"int":  "eksin.aws",
			"qa":   "vaapps",
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
			if env == "int" {
				tld = "corp"
			}
		}

		url := fmt.Sprintf("https://%s.exotec.%s", projectEnv, tld)
		if docs {
			url = fmt.Sprintf("%s/api/docs", url)
		}

		app.OpenCommand(url)

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success(fmt.Sprintf("Opening project: %s", url))

		return nil
	},
}
