package commands

import (
	"mycli/app"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
)

func TestConfigEditEditor(t *testing.T) {
	for _, source := range []string{"config", "visual", "editor", "nano", "invalid config"} {
		t.Run(source, func(t *testing.T) {
			bin := t.TempDir()
			name := "fake editor"
			if source == "nano" {
				name = "nano"
			}
			editor := filepath.Join(bin, name)
			if err := os.WriteFile(editor, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CONFIG_TEST_ARGS\"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(bin, "args")
			t.Setenv("CONFIG_TEST_ARGS", log)
			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", "")
			command := shellquote.Join(editor, "--wait")
			config := app.Config{}
			switch source {
			case "config":
				config.TextEditor = command
				t.Setenv("VISUAL", "nonexistent-visual-editor")
				t.Setenv("EDITOR", "nonexistent-environment-editor")
			case "visual":
				t.Setenv("VISUAL", command)
				t.Setenv("EDITOR", "nonexistent-environment-editor")
			case "editor", "invalid config":
				t.Setenv("EDITOR", command)
			case "nano":
				t.Setenv("PATH", bin)
			}
			home := dailyTestConfig(t, config)
			path := filepath.Join(home, app.CONFIG_FILE)
			if source == "invalid config" {
				if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := ConfigEdit.Before(nil); err != nil {
				t.Fatal(err)
			}
			if err := ConfigEdit.Action(nil); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(log)
			want := path + "\n"
			if source != "nano" {
				want = "--wait\n" + want
			}
			if err != nil || string(got) != want {
				t.Fatalf("editor args = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestConfigEditInvalidEditor(t *testing.T) {
	dailyTestConfig(t, app.Config{TextEditor: "'unclosed"})
	if err := ConfigEdit.Action(nil); err == nil || !strings.Contains(err.Error(), "invalid editor command") {
		t.Fatalf("expected invalid editor error, got %v", err)
	}
}
