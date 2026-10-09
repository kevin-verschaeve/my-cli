package commands

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/symfony-cli/console"
)

func TestHotfixCommandRegistration(t *testing.T) {
	registered := 0
	for _, command := range CommonCommands() {
		if command == Hotfix {
			registered++
		}
	}
	if registered != 1 {
		t.Fatalf("Hotfix registered %d times, want exactly once", registered)
	}
	if Hotfix.Name != "hotfix" {
		t.Fatalf("unexpected command name: %q", Hotfix.Name)
	}
	if Hotfix.Usage != "Hotfix workflow: start, publish, finish, resume, or status; omit the action for an interactive menu" {
		t.Fatalf("unexpected command usage: %q", Hotfix.Usage)
	}
	wantArgs := console.ArgDefinition{
		{Name: "action", Optional: true, Description: "start, publish, finish, resume, or status"},
	}
	if !reflect.DeepEqual(Hotfix.Args, wantArgs) {
		t.Fatalf("command args = %+v, want %+v", Hotfix.Args, wantArgs)
	}
	if Hotfix.Action == nil {
		t.Fatal("hotfix command must have an action")
	}
}

func TestHotfixCommandActionLoadsConfiguration(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYCLI__HOME", home)
	flags := flag.NewFlagSet("hotfix", flag.ContinueOnError)
	if err := flags.Parse([]string{"status"}); err != nil {
		t.Fatal(err)
	}
	context := console.NewContext(nil, flags, nil)
	context.Command = Hotfix
	if Hotfix.Action == nil {
		t.Fatal("hotfix command must have an action")
	}
	err := Hotfix.Action(context)
	var syntaxError *json.SyntaxError
	if err == nil || !strings.Contains(err.Error(), "unable to load configuration") || !errors.As(err, &syntaxError) {
		t.Fatalf("expected wrapped configuration JSON error from the hotfix action, got %v", err)
	}
}
