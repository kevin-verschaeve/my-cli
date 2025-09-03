package commands

import (
	"fmt"
	"mycli/app"

	"github.com/atotto/clipboard"
	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// Token will fetch a token on Azure and copy it to the clipboard
var Token = &console.Command{
	Name:  "token",
	Usage: "Fetch a token for the given app on Azure",
	Args: console.ArgDefinition{
		{Name: "app-name", Optional: false, Description: "The app name to get a token."},
	},
	Action: func(c *console.Context) error {
		appName := c.Args().Get("app-name")

		config, err := app.LoadConfig()

		if err != nil {
			panic(err)
		}

		appli, exists := config.GetApplication(appName)

		if !exists {
			panic("Application does not exists")
		}

		data := map[string]string{
			"grant_type":    "client_credentials",
			"client_id":     appli.ClientID,
			"client_secret": appli.ClientSecret,
			"scope":         appli.Scope,
		}

		response, err := app.GetAzureToken(data)

		if err != nil {
			panic(err)
		}

		err = clipboard.WriteAll(response.AccessToken)
		if err != nil {
			fmt.Println(response.AccessToken)
			panic(fmt.Errorf("unable to copy to clipboard"))
		}

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success("Token copié dans le presse papier")

		return nil
	},
}
