package commands

import (
	"encoding/json"
	"fmt"
	"mycli/app"
	"os"
	"path/filepath"

	"github.com/symfony-cli/console"
)

// Init creates the configuration without overwriting an existing file.
var Init = &console.Command{
	Name:  "init",
	Usage: "Create the default configuration",
	Action: func(c *console.Context) error {
		config := app.Config{
			VersionControlService: "gitlab",
			DailyFile:             "daily.txt",
			Hotfix: app.HotfixConfig{
				Prefix:          "hotfix",
				BackportTargets: []string{},
				BackportMode:    "review",
			},
			Applications: map[string]app.ApplicationConfig{
				"pily": {},
				"pq":   {},
			},
		}
		data, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return fmt.Errorf("unable to encode configuration: %w", err)
		}
		home := app.MyCliHome()
		if err := os.MkdirAll(home, 0700); err != nil {
			return fmt.Errorf("unable to create configuration directory: %w", err)
		}
		path := filepath.Join(home, app.CONFIG_FILE)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("unable to create configuration at %s: %w", path, err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			file.Close()
			os.Remove(path)
			return fmt.Errorf("unable to write configuration: %w", err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("unable to close configuration: %w", err)
		}
		fmt.Printf("Configuration created at %s. Use config:edit to customize it.\n", path)
		return nil
	},
}
