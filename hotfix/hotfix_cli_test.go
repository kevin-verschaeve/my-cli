package hotfix

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

func hotfixTestConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"hotfix":{"prefix":"hotfix"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYCLI__HOME", home)
}

func hotfixTestAction(t *testing.T, action string) error {
	t.Helper()
	flags := flag.NewFlagSet("hotfix", flag.ContinueOnError)
	if err := flags.Parse([]string{action}); err != nil {
		t.Fatal(err)
	}
	context := console.NewContext(nil, flags, nil)
	context.Command = &console.Command{
		Name: "hotfix",
		Args: console.ArgDefinition{
			{Name: "action", Optional: true, Description: "start, publish, finish, resume, or status"},
		},
		Action: Run,
	}
	return context.Command.Action(context)
}

func TestHotfixStatusWorksOfflineWithDirtyTree(t *testing.T) {
	git := hotfixTestRepo(t)
	hotfixTestConfig(t)
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.LastError = "previous failure"
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	git("remote", "remove", "origin")
	if err := os.WriteFile("untracked", []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := hotfixTestAction(t, "status"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(store.path)
	if err != nil || string(before) != string(after) || git("branch", "--show-current") != "main" {
		t.Fatal("status must be read-only and work without origin or a clean tree")
	}
}

func TestHotfixResumeRestoresInitialBranch(t *testing.T) {
	git := hotfixTestRepo(t)
	hotfixTestConfig(t)
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	git("commit", "--allow-empty", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	git("push", "origin", "hotfix/1.0.1")
	git("checkout", "main")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.Base, state.BaseCommit, state.Commit = "1.0.0", base, commit
	state.Phase, state.TagStatus, state.BackportsPlanned = "finishing", "skipped", true
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "direct", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	if err := hotfixTestAction(t, "resume"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state = reloaded.Workflows[state.Branch]
	if state.Phase != "completed" || state.TagStatus != "skipped" || state.Backports[0].Phase != "done" {
		t.Fatalf("unexpected finished state: %+v", state)
	}
	if git("branch", "--show-current") != "main" {
		t.Fatal("successful resume must restore initial branch")
	}
}

func TestHotfixResumeRecordsPreflightFailure(t *testing.T) {
	hotfixTestRepo(t)
	hotfixTestConfig(t)
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.Phase = "finishing"
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("untracked", []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := hotfixTestAction(t, "resume"); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("expected preflight rejection, got %v", err)
	}
	reloaded, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reloaded.Workflows[state.Branch].LastError, "not clean") {
		t.Fatal("failure must be recorded on the resumed hotfix, not the current branch")
	}
}

func TestHotfixPublishWithoutReviewOrCIPrompts(t *testing.T) {
	git := hotfixTestRepo(t)
	hotfixTestConfig(t)
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	git("commit", "--allow-empty", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.Base, state.BaseCommit = "1.0.0", base
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	// Publishing must succeed with no terminal input or configured VCS client.
	for i := 0; i < 2; i++ {
		if err := hotfixTestAction(t, "publish"); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state = reloaded.Workflows[state.Branch]
	if state.Phase != "published" || state.Commit != commit || git("rev-parse", "origin/hotfix/1.0.1") != commit {
		t.Fatalf("publication must still push and checkpoint the fix: %+v", state)
	}
}

func TestHotfixFinishWithoutReviewOrCI(t *testing.T) {
	for _, vcs := range []string{"", "github", "gitlab"} {
		t.Run("vcs="+vcs, func(t *testing.T) {
			git := hotfixTestRepo(t)
			hotfixTestConfig(t)
			configPath := filepath.Join(os.Getenv("MYCLI__HOME"), "config.json")
			if err := os.WriteFile(configPath, []byte(`{"vcs":"`+vcs+`","hotfix":{"prefix":"hotfix"}}`), 0600); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			for _, tool := range []string{"gh", "glab"} {
				if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\necho 'review API must not be called' >&2\nexit 1\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			base := git("rev-parse", "HEAD")
			git("checkout", "-b", "hotfix/1.0.1")
			git("commit", "--allow-empty", "-m", "Fix")
			commit := git("rev-parse", "HEAD")
			git("push", "origin", "hotfix/1.0.1")
			store, err := hotfixLoadStore()
			if err != nil {
				t.Fatal(err)
			}
			state := store.state("hotfix/1.0.1")
			state.Base, state.BaseCommit, state.Commit = "1.0.0", base, commit
			state.Phase, state.TagStatus, state.BackportsPlanned = "published", "skipped", true
			if err := store.save(); err != nil {
				t.Fatal(err)
			}
			// Only release confirmation is required; no review or CI evidence exists.
			previousInput := terminal.Stdin
			terminal.Stdin = terminal.NewInput(strings.NewReader("yes\n"))
			terminal.Stdin.SetInteractive(true)
			t.Cleanup(func() { terminal.Stdin = previousInput })
			if err := hotfixTestAction(t, "finish"); err != nil {
				t.Fatal(err)
			}
			reloaded, err := hotfixLoadStore()
			if err != nil {
				t.Fatal(err)
			}
			state = reloaded.Workflows[state.Branch]
			if state.Phase != "completed" || state.Commit != commit || state.TagStatus != "skipped" {
				t.Fatalf("finalization without review must complete: %+v", state)
			}
		})
	}
}

func TestHotfixConfirm(t *testing.T) {
	previousInput := terminal.Stdin
	terminal.Stdin = terminal.NewInput(strings.NewReader("invalid\nyes\n\nno\n"))
	terminal.Stdin.SetInteractive(true)
	t.Cleanup(func() { terminal.Stdin = previousInput })
	reader := bufio.NewReader(terminal.Stdin)
	for _, want := range []bool{true, false, false} {
		got, err := hotfixConfirm(reader, "Confirm?")
		if err != nil || got != want {
			t.Fatalf("confirmation = %v, %v; want %v", got, err, want)
		}
	}
	if got, err := hotfixConfirm(reader, "Confirm?"); got || !errors.Is(err, io.EOF) {
		t.Fatalf("exhausted input must stop without approval: %v, %v", got, err)
	}
	terminal.Stdin.SetInteractive(false)
	if got, err := hotfixConfirm(reader, "Confirm?"); got || err != nil {
		t.Fatalf("noninteractive confirmation must default to no: %v, %v", got, err)
	}
}

func TestHotfixFinishTagDescription(t *testing.T) {
	for _, scenario := range []string{"generated", "edited", "resumed", "editor-failure", "input-exhausted"} {
		t.Run(scenario, func(t *testing.T) {
			git := hotfixTestRepo(t)
			hotfixTestConfig(t)
			base := git("rev-parse", "HEAD")
			git("checkout", "-b", "hotfix/1.0.1")
			git("commit", "--allow-empty", "-m", "Fix checkout timeout")
			commit := git("rev-parse", "HEAD")
			git("push", "origin", "hotfix/1.0.1")
			store, err := hotfixLoadStore()
			if err != nil {
				t.Fatal(err)
			}
			state := store.state("hotfix/1.0.1")
			state.Base, state.BaseCommit, state.Commit = "1.0.0", base, commit
			state.Phase, state.BackportsPlanned = "published", true
			want := "Hotfix release 1.0.1\n\n- Fix checkout timeout"
			input := "yes\nyes\nno\n"
			if scenario == "input-exhausted" {
				input = "yes\n"
			}
			if scenario == "resumed" {
				want = "Custom description saved before retry"
				state.Phase, state.TagStatus, state.TagMessage = "finishing", "pending", want
				input = ""
			}
			if scenario == "edited" || scenario == "editor-failure" {
				input = "yes\nyes\nyes\n"
				want = "Hotfix 1.0.1\n\n- Restore checkout"
				editor := filepath.Join(t.TempDir(), "editor")
				script := "#!/bin/sh\ngrep -q 'Fix checkout timeout' \"$1\" || exit 1\nprintf 'Hotfix 1.0.1\\n\\n- Restore checkout\\n' > \"$1\"\n"
				if scenario == "editor-failure" {
					script = "#!/bin/sh\nexit 1\n"
				}
				if err := os.WriteFile(editor, []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("VISUAL", editor)
				// Survey reads OS stdin to launch the editor; Symfony prompts use terminal.Stdin.
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := writer.WriteString("\n"); err != nil {
					t.Fatal(err)
				}
				writer.Close()
				previousInput := os.Stdin
				os.Stdin = reader
				t.Cleanup(func() { os.Stdin = previousInput; reader.Close() })
			}
			if err := store.save(); err != nil {
				t.Fatal(err)
			}
			previousInput := terminal.Stdin
			terminal.Stdin = terminal.NewInput(strings.NewReader(input))
			terminal.Stdin.SetInteractive(true)
			t.Cleanup(func() { terminal.Stdin = previousInput })
			err = hotfixTestAction(t, "finish")
			if scenario == "input-exhausted" {
				if !errors.Is(err, io.EOF) || git("tag", "--list", "1.0.1") != "" || git("ls-remote", "origin", "refs/tags/1.0.1") != "" {
					t.Fatalf("exhausted input must stop before tag creation or publication: %v", err)
				}
				reloaded, err := hotfixLoadStore()
				if err != nil {
					t.Fatal(err)
				}
				if saved := reloaded.Workflows[state.Branch]; saved.Phase != "finishing" || saved.TagStatus != "" {
					t.Fatalf("missing input must leave the tag decision resumable: %+v", saved)
				}
				return
			}
			if scenario == "editor-failure" {
				if err == nil || git("tag", "--list", "1.0.1") != "" || git("ls-remote", "origin", "refs/tags/1.0.1") != "" {
					t.Fatalf("editor failure must stop before tag creation or publication: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := git("for-each-ref", "--format=%(contents)", "refs/tags/1.0.1"); got != want {
				t.Fatalf("tag description = %q, want %q", got, want)
			}
			reloaded, err := hotfixLoadStore()
			if err != nil {
				t.Fatal(err)
			}
			state = reloaded.Workflows[state.Branch]
			if state.TagMessage != want || state.TagStatus != "published" || state.Phase != "completed" {
				t.Fatalf("description must be saved with the published tag: %+v", state)
			}
			if git("ls-remote", "origin", "refs/tags/1.0.1") == "" {
				t.Fatal("release tag must be pushed")
			}
		})
	}
}
