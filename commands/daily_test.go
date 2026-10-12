package commands

import (
	"encoding/json"
	"flag"
	"mycli/app"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/symfony-cli/console"
)

func dailyTestConfig(t *testing.T, config app.Config) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv(app.EnvPrefix+"__HOME", home)
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, app.CONFIG_FILE), data, 0600); err != nil {
		t.Fatal(err)
	}
	return home
}

func runDailyTest(t *testing.T, args ...string) (string, error) {
	t.Helper()
	flags := flag.NewFlagSet("daily", flag.ContinueOnError)
	flags.Bool("ui", false, "")
	if err := flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	context := console.NewContext(nil, flags, nil)
	context.Command = Daily
	output, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	stdout := os.Stdout
	os.Stdout = output
	defer func() { os.Stdout = stdout }()
	err = Daily.Action(context)
	content, readErr := os.ReadFile(output.Name())
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(content), err
}

func TestDailyFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv(app.EnvPrefix+"__HOME", filepath.Join(home, "config"))
	t.Setenv("HOME", home)
	date := time.Date(2026, 1, 2, 23, 30, 0, 0, time.FixedZone("local", -7200))
	for _, test := range []struct {
		dir  string
		want string
	}{
		{"", filepath.Join(home, "config", "daily", "2026-01-02.md")},
		{"notes", filepath.Join(home, "config", "notes", "2026-01-02.md")},
		{filepath.Join(home, "absolute"), filepath.Join(home, "absolute", "2026-01-02.md")},
		{"~/notes", filepath.Join(home, "notes", "2026-01-02.md")},
		{"~", filepath.Join(home, "2026-01-02.md")},
	} {
		t.Run(test.dir, func(t *testing.T) {
			got, err := dailyFilePath(test.dir, date)
			if err != nil || got != test.want {
				t.Fatalf("dailyFilePath(%q) = %q, %v; want %q", test.dir, got, err, test.want)
			}
		})
	}
}

func TestDailyAppendAndDisplay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "daily notes")
	dailyTestConfig(t, app.Config{DailyDir: dir})
	for _, args := range [][]string{{"--", "- First task"}, {"--", "-", "Second", "task"}} {
		if _, err := runDailyTest(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, time.Now().Format("2006-01-02")+".md")
	want := "- First task\n- Second task\n"
	content, err := os.ReadFile(path)
	if err != nil || string(content) != want {
		t.Fatalf("daily content = %q, %v; want %q", content, err, want)
	}
	previous := filepath.Join(dir, "2000-01-01.md")
	if err := os.WriteFile(previous, []byte("previous day\n"), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := runDailyTest(t)
	wantDisplay := filepath.Base(path) + "\n\n" + want
	if err != nil || output != wantDisplay {
		t.Fatalf("display = %q, %v; want %q", output, err, wantDisplay)
	}
	content, err = os.ReadFile(previous)
	if err != nil || string(content) != "previous day\n" {
		t.Fatalf("previous file changed: %q, %v", content, err)
	}
}

func TestDailyDisplayMissingFile(t *testing.T) {
	home := dailyTestConfig(t, app.Config{})
	output, err := runDailyTest(t)
	if err != nil || output != "" {
		t.Fatalf("missing daily display = %q, %v", output, err)
	}
	if _, err := os.Stat(filepath.Join(home, "daily")); !os.IsNotExist(err) {
		t.Fatalf("display should not create a daily directory: %v", err)
	}
}

func TestDailyEditor(t *testing.T) {
	for _, source := range []string{"config", "environment", "nano"} {
		t.Run(source, func(t *testing.T) {
			bin := t.TempDir()
			name := "fake editor"
			if source == "nano" {
				name = "nano"
			}
			editor := filepath.Join(bin, name)
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DAILY_TEST_ARGS\"\nfor path do :; done\nprintf '%s\\n' '- Edited task' >> \"$path\"\n"
			if err := os.WriteFile(editor, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(bin, "args")
			t.Setenv("DAILY_TEST_ARGS", log)
			t.Setenv("EDITOR", "")
			t.Setenv("VISUAL", "nonexistent-visual-editor")
			config := app.Config{}
			command := shellquote.Join(editor, "--wait", "argument with spaces")
			switch source {
			case "config":
				config.TextEditor = command
				t.Setenv("EDITOR", "nonexistent-environment-editor")
			case "environment":
				t.Setenv("EDITOR", command)
			case "nano":
				t.Setenv("PATH", bin)
			}
			home := dailyTestConfig(t, config)
			path := filepath.Join(home, "daily", time.Now().Format("2006-01-02")+".md")
			if _, err := runDailyTest(t, "--ui"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(log)
			want := []string{path}
			if source != "nano" {
				want = append([]string{"--wait", "argument with spaces"}, want...)
			}
			if err != nil || !reflect.DeepEqual(strings.Split(strings.TrimSuffix(string(got), "\n"), "\n"), want) {
				t.Fatalf("editor args = %q, %v; want %q", got, err, want)
			}
			// Opening an existing file must preserve its content.
			if _, err := runDailyTest(t, "--ui"); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(path)
			if err != nil || string(content) != "- Edited task\n- Edited task\n" {
				t.Fatalf("edited daily content = %q, %v", content, err)
			}
		})
	}
}

func TestDailyErrors(t *testing.T) {
	t.Run("configuration", func(t *testing.T) {
		home := dailyTestConfig(t, app.Config{})
		if err := os.WriteFile(filepath.Join(home, app.CONFIG_FILE), []byte("{broken"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := runDailyTest(t); err == nil || !strings.Contains(err.Error(), "unable to load configuration") {
			t.Fatalf("expected configuration error, got %v", err)
		}
	})
	t.Run("ui with text", func(t *testing.T) {
		dailyTestConfig(t, app.Config{})
		if _, err := runDailyTest(t, "--ui", "task"); err == nil {
			t.Fatal("expected incompatible arguments error")
		}
	})
	t.Run("directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(path, nil, 0644); err != nil {
			t.Fatal(err)
		}
		dailyTestConfig(t, app.Config{DailyDir: path})
		if _, err := runDailyTest(t, "task"); err == nil {
			t.Fatal("expected directory error")
		}
	})
	for _, editor := range []string{"'unclosed", "   ", "nonexistent-daily-test-editor", "sh -c 'exit 7'"} {
		t.Run(editor, func(t *testing.T) {
			dailyTestConfig(t, app.Config{TextEditor: editor})
			if _, err := runDailyTest(t, "--ui"); err == nil {
				t.Fatal("expected editor error")
			}
		})
	}
}
