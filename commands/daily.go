package commands

import (
	"fmt"
	"mycli/app"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/symfony-cli/console"
)

func dailyFilePath(dir string, date time.Time) (string, error) {
	if dir == "" {
		dir = "daily"
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(app.MyCliHome(), dir)
	}
	return filepath.Join(dir, date.Format("2006-01-02")+".md"), nil
}

// Daily appends entries to today's Markdown file, displays it, or opens an editor.
var Daily = &console.Command{
	Name:    "daily",
	Aliases: []*console.Alias{{Name: "d"}},
	Usage:   "Add an entry to today's Markdown daily, or display its content",
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:  "ui",
			Usage: "Open today's file in text_editor, $EDITOR, or nano (without task text)",
		},
	},
	Args: console.ArgDefinition{
		{
			Name:        "task",
			Optional:    true,
			Description: "Text to append, or nothing to display today's file",
			Slice:       true,
		},
	},
	Action: func(c *console.Context) error {
		task := c.Args().Tail()
		if c.Bool("ui") && len(task) > 0 {
			return fmt.Errorf("--ui cannot be used with task text")
		}
		config, err := app.LoadConfig()
		if err != nil {
			return fmt.Errorf("unable to load configuration: %w", err)
		}
		path, err := dailyFilePath(config.DailyDir, time.Now())
		if err != nil {
			return err
		}

		if len(task) == 0 && !c.Bool("ui") {
			content, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(os.Stdout, "%s\n\n", filepath.Base(path)); err != nil {
				return err
			}
			_, err = os.Stdout.Write(content)
			return err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if len(task) > 0 {
			if _, err := file.WriteString(strings.Join(task, " ") + "\n"); err != nil {
				file.Close()
				return err
			}
		}
		if err := file.Close(); err != nil {
			return err
		}
		if !c.Bool("ui") {
			return nil
		}

		editor := config.TextEditor
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "nano"
		}
		args, err := shellquote.Split(editor)
		if err != nil || len(args) == 0 {
			return fmt.Errorf("invalid editor command: %q", editor)
		}
		cmd := exec.Command(args[0], append(args[1:], path)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	},
}
