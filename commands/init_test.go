package commands

import (
	"encoding/json"
	"errors"
	"mycli/app"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInitCommandRegistration(t *testing.T) {
	count := 0
	for _, command := range CommonCommands() {
		if command == Init {
			count++
		}
	}
	if count != 1 || Init.Name != "init" || Init.Action == nil {
		t.Fatalf("invalid init command registration: count=%d, command=%+v", count, Init)
	}
}

func TestInitCreatesDefaultConfiguration(t *testing.T) {
	home := filepath.Join(t.TempDir(), "nested", "config")
	t.Setenv(app.EnvPrefix+"__HOME", home)
	if err := Init.Action(nil); err != nil {
		t.Fatal(err)
	}
	config, err := app.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../config.json.dist")
	if err != nil {
		t.Fatal(err)
	}
	var want app.Config
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*config, want) {
		t.Fatalf("configuration = %+v, want %+v", *config, want)
	}
	info, err := os.Stat(filepath.Join(home, app.CONFIG_FILE))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("configuration permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestInitDoesNotOverwriteConfiguration(t *testing.T) {
	home := t.TempDir()
	t.Setenv(app.EnvPrefix+"__HOME", home)
	path := filepath.Join(home, app.CONFIG_FILE)
	want := []byte(`{"vcs":"github"}`)
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Init.Action(nil); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected existing file error, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("existing configuration changed: %s", got)
	}
}

func TestInitReportsDirectoryError(t *testing.T) {
	home := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(home, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(app.EnvPrefix+"__HOME", home)
	if err := Init.Action(nil); err == nil {
		t.Fatal("expected configuration directory error")
	}
}
