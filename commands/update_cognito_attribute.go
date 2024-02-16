package commands

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

// UpdateCognitoUserAttribute update a given attribute for a given user on a specific cognito pool.
var UpdateCognitoUserAttribute = &console.Command{
	// hide the command as we don't use cognito here
	Hidden: func() bool {
		return true
	},
	Name:    "aws:cognito:update-user-attribute",
	Aliases: []*console.Alias{{Name: "attr"}},
	Usage:   "Update a user attribute on the given pool for the given user",
	Args: console.ArgDefinition{
		{Name: "pool", Optional: false, Description: "The cognito pool id"},
		{Name: "username", Optional: false, Description: "User identifier on cognito"},
		{Name: "attribute", Optional: false, Description: "The attribute you want to update"},
		{Name: "value", Optional: false, Description: "The new value to set"},
	},
	Action: func(c *console.Context) error {
		sess := session.Must(session.NewSession(&aws.Config{
			Region: aws.String("eu-west-1"),
		}))
		svc := cognitoidentityprovider.New(sess)

		_, err := svc.AdminUpdateUserAttributes(&cognitoidentityprovider.AdminUpdateUserAttributesInput{
			UserAttributes: []*cognitoidentityprovider.AttributeType{
				{
					Name:  aws.String(c.Args().Get("attribute")),
					Value: aws.String(c.Args().Get("value")),
				},
			},
			UserPoolId: aws.String(c.Args().Get("pool")),
			Username:   aws.String(c.Args().Get("username")),
		})

		if err != nil {
			return err
		}

		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		ui.Success("Attribute successfully updated")

		return nil
	},
}
