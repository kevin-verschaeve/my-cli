package hotfix

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/symfony-cli/console"
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
