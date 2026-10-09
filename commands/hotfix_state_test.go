package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHotfixStoreRoundTrip(t *testing.T) {
	hotfixTestRepo(t)
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.Phase, state.Commit, state.TagStatus = "finishing", "abc", "published"
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "direct", Phase: "done"}}
	if err := store.save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := hotfixLoadStore()
	if err != nil || !reflect.DeepEqual(store.Workflows, reloaded.Workflows) {
		t.Fatalf("checkpoint round trip failed: %v", err)
	}
	if hotfixDefaultAction(state) != "finish" {
		t.Fatal("unfinished finalization must resume at finish")
	}
	if err := os.WriteFile(store.path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := hotfixLoadStore(); err == nil {
		t.Fatal("corrupt checkpoints must not be silently discarded")
	}
}

func TestHotfixBackportTargets(t *testing.T) {
	git := hotfixTestRepo(t)
	git("branch", "develop")
	git("push", "origin", "develop")
	got, err := hotfixBackportTargets(nil)
	if err != nil || !reflect.DeepEqual(got, []string{"develop", "main"}) {
		t.Fatalf("targets = %v, %v", got, err)
	}
	got, err = hotfixBackportTargets([]string{"main", "main"})
	if err != nil || !reflect.DeepEqual(got, []string{"main"}) {
		t.Fatalf("deduplicated targets = %v, %v", got, err)
	}
	if _, err := hotfixBackportTargets([]string{"missing"}); err == nil {
		t.Fatal("missing targets must be rejected")
	}
}

func TestHotfixReviewBackportAndResume(t *testing.T) {
	git := hotfixTestRepo(t)
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	if err := os.WriteFile("fix.txt", []byte("fix"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "fix.txt")
	git("commit", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	git("push", "origin", "hotfix/1.0.1")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.BaseCommit, state.Commit, state.Phase = base, commit, "finishing"
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "review", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	// A failed provider action must preserve the pushed backport and never push main.
	if err := hotfixRunBackport(store, state, 0, ""); err == nil {
		t.Fatal("missing review tool must report an error")
	}
	if state.Backports[0].Phase != "pushed" || git("rev-parse", "origin/main") != base {
		t.Fatal("review backport must push a separate branch, never its target")
	}
	if git("show", "origin/backport/hotfix/1.0.1/main:fix.txt") != "fix" {
		t.Fatal("fix was not backported")
	}
	prepared := git("rev-parse", "HEAD")
	// Stub the external CLI only; all Git commands still use real repositories.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nif [ \"$2\" = \"list\" ]; then printf '[]\\n'; else printf 'https://example.test/pr/1\\n'; fi\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	reloaded, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state = reloaded.Workflows[state.Branch]
	if err := hotfixRunBackport(reloaded, state, 0, "github"); err != nil {
		t.Fatal(err)
	}
	if state.Backports[0].Phase != "done" || git("rev-parse", "HEAD") != prepared {
		t.Fatal("resuming must not duplicate cherry-picks")
	}
	git("checkout", "main")
	if err := hotfixRunBackport(reloaded, state, 0, "github"); err != nil {
		t.Fatal(err)
	}
	if git("branch", "--show-current") != "main" {
		t.Fatal("completed backports must be skipped without changing branch")
	}
}

func TestHotfixConflictResume(t *testing.T) {
	git := hotfixTestRepo(t)
	if err := os.WriteFile("shared.txt", []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "shared.txt")
	git("commit", "-m", "Base file")
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	if err := os.WriteFile("shared.txt", []byte("fixed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "Fix")
	commit := git("rev-parse", "HEAD")
	git("checkout", "main")
	if err := os.WriteFile("shared.txt", []byte("target change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "Target change")
	git("push", "origin", "main")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.BaseCommit, state.Commit = base, commit
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "direct", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	if err := hotfixRunBackport(store, state, 0, ""); err == nil || !strings.Contains(err.Error(), "--continue") {
		t.Fatalf("expected actionable conflict, got %v", err)
	}
	if state.Backports[0].Phase != "applying" {
		t.Fatal("conflict must retain applying checkpoint")
	}
	if err := hotfixEnsureClean(); err == nil {
		t.Fatal("unresolved conflict must block resume")
	}
	if err := os.WriteFile("shared.txt", []byte("fixed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "shared.txt")
	git("-c", "core.editor=true", "merge", "--continue")
	if err := hotfixRunBackport(store, state, 0, ""); err != nil {
		t.Fatal(err)
	}
	if state.Backports[0].Phase != "done" || git("show", "origin/main:shared.txt") != "fixed" {
		t.Fatal("resolved backport must be pushed on resume")
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", commit, "origin/main")
	if err := cmd.Run(); err != nil {
		t.Fatal("resolved backport lost the validated hotfix commit")
	}
}

func TestHotfixReviewConflictResume(t *testing.T) {
	git := hotfixTestRepo(t)
	if err := os.WriteFile("shared.txt", []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "shared.txt")
	git("commit", "-m", "Base file")
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	if err := os.WriteFile("shared.txt", []byte("fix\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "Fix")
	commit := git("rev-parse", "HEAD")
	git("checkout", "main")
	if err := os.WriteFile("shared.txt", []byte("target\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("commit", "-am", "Target change")
	git("push", "origin", "main")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.BaseCommit, state.Commit = base, commit
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "review", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	if err := hotfixRunBackport(store, state, 0, ""); err == nil || !strings.Contains(err.Error(), "cherry-pick --continue") {
		t.Fatalf("expected cherry-pick conflict, got %v", err)
	}
	// A conflict resolution may differ from the original patch, so git cherry
	// alone cannot reliably detect that it has already been applied.
	if err := os.WriteFile("shared.txt", []byte("fix plus target\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "shared.txt")
	git("-c", "core.editor=true", "cherry-pick", "--continue")
	resolved := git("rev-parse", "HEAD")
	if err := hotfixRunBackport(store, state, 0, ""); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("expected only missing review tool error, got %v", err)
	}
	if state.Backports[0].Phase != "pushed" || git("rev-parse", "HEAD") != resolved {
		t.Fatal("resolved cherry-pick was duplicated on resume")
	}
}

func TestHotfixReviewBackportAlreadyPresent(t *testing.T) {
	git := hotfixTestRepo(t)
	base := git("rev-parse", "HEAD")
	git("checkout", "-b", "hotfix/1.0.1")
	if err := os.WriteFile("fix.txt", []byte("fix"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "fix.txt")
	git("commit", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	git("checkout", "main")
	git("cherry-pick", commit)
	git("push", "origin", "main")
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.BaseCommit, state.Commit = base, commit
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "review", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	if err := hotfixRunBackport(store, state, 0, ""); err != nil {
		t.Fatal(err)
	}
	if state.Backports[0].Phase != "done" || git("branch", "-r", "--list", "origin/backport/*") != "" {
		t.Fatal("an already-present patch must not create an empty review")
	}
}

func TestHotfixBackportPushRetry(t *testing.T) {
	git := hotfixTestRepo(t)
	git("checkout", "-b", "hotfix/1.0.1")
	git("commit", "--allow-empty", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	remote := git("remote", "get-url", "origin")
	hook := filepath.Join(remote, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'push rejected for test' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	store, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state := store.state("hotfix/1.0.1")
	state.Commit = commit
	state.Backports = []hotfixBackportState{{Target: "main", Mode: "direct", Branch: "backport/hotfix/1.0.1/main", Phase: "pending"}}
	if err := hotfixRunBackport(store, state, 0, ""); err == nil || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("expected retryable push error, got %v", err)
	}
	prepared := git("rev-parse", "HEAD")
	if state.Backports[0].Phase != "applied" {
		t.Fatal("failed push must preserve prepared result")
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	reloaded, err := hotfixLoadStore()
	if err != nil {
		t.Fatal(err)
	}
	state = reloaded.Workflows[state.Branch]
	if err := hotfixRunBackport(reloaded, state, 0, ""); err != nil {
		t.Fatal(err)
	}
	if state.Backports[0].Phase != "done" || git("rev-parse", "origin/main") != prepared || git("rev-parse", "HEAD") != prepared {
		t.Fatal("retry must publish prepared merge without duplicating it")
	}
}
