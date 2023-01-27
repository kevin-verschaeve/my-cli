package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// OpenCommand find the best command to open a file or url depending on the OS.
func OpenCommand(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}

// GetEnv get an environment variable or return a default value if not found.
func GetEnv(varname, defaultValue string) string {
	value, exists := os.LookupEnv(fmt.Sprintf("MYCLI__%s", varname))
	if !exists {
		return defaultValue
	}

	return value
}

// CheckCommandExists look for a command in the os and returns an error if it does not exists.
func CheckCommandExists(cmd string) error {
	_, err := exec.LookPath(cmd)

	return err
}

func RunGitCommand(arguments ...string) (string, error) {
	cmd, b, e := exec.Command("git", arguments...), new(strings.Builder), new(strings.Builder)
	cmd.Stdout = b
	cmd.Stderr = e
	err := cmd.Run()

	if err != nil {
		return b.String(), errors.New(e.String())
	}

	return b.String(), nil
}

func MyCliHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	return GetEnv("HOME", home+"/mycli")
}
