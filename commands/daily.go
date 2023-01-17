package commands

import (
	"fmt"
	"io/ioutil"
	"log"
	"mycli/app"
	"os"
	"strings"
	"time"

	"github.com/symfony-cli/console"
)

// Daily allows to store some text in a file that is reused for the daily of the next days.
// To fully enjoy this feature, you will need to add 2 crons.
//
// # Display daily content at 10am, on weekdays.
// 0 10 * * 1-5 DISPLAY=:0 google-chrome ~/.sb/daily
//
// # Clean daily file after each daily at 10:30am, on weekdays.
// 30 10 * * 1-5 date > ~/daily.txt
//
// Feel free to adjust the time of those crons to better fit your needs.
var Daily = &console.Command{
	Name:    "daily",
	Aliases: []*console.Alias{{Name: "d"}},
	Usage:   "Add line entry in daily for tomorrow",
	Flags: []console.Flag{
		&console.BoolFlag{
			Name:     "ui",
			Required: false,
			Usage:    "Open daily content in a gui application (works only when no text is given)",
		},
	},
	Args: console.ArgDefinition{
		{
			Name:        "task",
			Optional:    true,
			Description: "The task done you want to save or nothing to display the content of the file",
			Slice:       true,
		},
	},
	Before: func(c *console.Context) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		dir := app.GetConfig("DailyDirectory")

		if _, err := os.Stat(fmt.Sprintf("%s/%s", home, dir)); os.IsNotExist(err) {
			if err := os.Mkdir(fmt.Sprintf("%s/%s", home, dir), os.ModePerm); err != nil {
				return err
			}
		}

		return nil
	},
	Action: func(c *console.Context) error {
		task := c.Args().Tail()
		home, _ := os.UserHomeDir()
		dir := app.GetConfig("DailyDirectory")
		dailyFile := app.GetConfig("DailyFile")

		var filePath string
		if dir == "" {
			filePath = fmt.Sprintf("%s/%s", home, dailyFile)
		} else {
			filePath = fmt.Sprintf("%s/%s/%s", home, dir, dailyFile)
		}

		if len(task) == 0 {
			if c.Bool("ui") {
				app.OpenCommand(filePath)

				return nil
			}

			file, _ := os.Open(filePath)
			b, _ := ioutil.ReadAll(file)
			fmt.Print(string(b))
			file.Close()

			return nil
		}

		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Println(err)
		}

		defer f.Close()

		fInfo, err := os.Stat(filePath)
		if err != nil {
			return err
		}

		fsize := fInfo.Size()
		if fsize <= 0 {
			currentTime := time.Now()
			header := currentTime.Format("Monday 02/01/2006")
			if _, err := f.WriteString(header + "\n\n"); err != nil {
				return err
			}
		}

		if _, err := f.WriteString(strings.Join(task, " ") + "\n"); err != nil {
			return err
		}

		return nil
	},
}
