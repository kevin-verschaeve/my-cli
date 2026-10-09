package hotfix

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests change the working directory, just like TestHotfixGitSteps.
func hotfixTestRepo(t *testing.T) func(...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	git("init", "--bare", remote)
	git("init")
	git("config", "user.name", "Hotfix Test")
	git("config", "user.email", "hotfix@example.test")
	git("config", "commit.gpgsign", "false")
	git("config", "tag.gpgsign", "false")
	git("remote", "add", "origin", remote)
	git("checkout", "-b", "main")
	git("commit", "--allow-empty", "-m", "Initial release")
	git("tag", "1.0.0")
	git("push", "origin", "main", "--tags")
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	return git
}

func TestHotfixPreflight(t *testing.T) {
	git := hotfixTestRepo(t)
	if err := hotfixPreflight(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("untracked", []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := hotfixEnsureClean(); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("expected dirty worktree error, got %v", err)
	}
	git("add", "untracked")
	if err := hotfixEnsureClean(); err == nil {
		t.Fatal("staged changes must block the workflow")
	}
	git("commit", "-m", "Work")
	if err := os.WriteFile(filepath.Join(".git", "MERGE_HEAD"), []byte(git("rev-parse", "HEAD")), 0600); err != nil {
		t.Fatal(err)
	}
	if err := hotfixEnsureClean(); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("expected merge-in-progress error, got %v", err)
	}
}

func TestHotfixValidateNewBranch(t *testing.T) {
	git := hotfixTestRepo(t)
	if err := hotfixValidateNewBranch("hotfix/1.0.1", "hotfix"); err != nil {
		t.Fatal(err)
	}
	for _, branch := range []string{"hotfix/custom", "hotfix/1.0.0"} {
		if err := hotfixValidateNewBranch(branch, "hotfix"); err == nil {
			t.Fatalf("%s must be rejected", branch)
		}
	}
	git("branch", "hotfix/1.0.1")
	if err := hotfixValidateNewBranch("hotfix/1.0.1", "hotfix"); err == nil {
		t.Fatal("existing local branch must be rejected")
	}
	git("push", "origin", "hotfix/1.0.1")
	git("branch", "-D", "hotfix/1.0.1")
	if err := hotfixValidateNewBranch("hotfix/1.0.1", "hotfix"); err == nil {
		t.Fatal("existing remote branch must be rejected")
	}
}

func TestHotfixPushTagUsesExplicitCommit(t *testing.T) {
	git := hotfixTestRepo(t)
	git("checkout", "-b", "hotfix/1.0.1")
	git("commit", "--allow-empty", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	git("push", "origin", "hotfix/1.0.1")
	if got, err := hotfixPublishedCommit("hotfix/1.0.1"); err != nil || got != commit {
		t.Fatalf("published commit = %s, %v", got, err)
	}
	git("checkout", "main")
	for i := 0; i < 2; i++ {
		if err := hotfixPushTag("1.0.1", commit); err != nil {
			t.Fatal(err)
		}
	}
	if got := git("rev-parse", "1.0.1^{commit}"); got != commit {
		t.Fatalf("tag = %s, want %s", got, commit)
	}
	if git("cat-file", "-t", "1.0.1") != "tag" {
		t.Fatal("release tag must be annotated")
	}
	git("checkout", "hotfix/1.0.1")
	git("commit", "--allow-empty", "-m", "Unreviewed fix")
	if _, err := hotfixPublishedCommit("hotfix/1.0.1"); err == nil {
		t.Fatal("unpublished commits must block finalization")
	}
}

func TestHotfixPushTagRejectsMismatches(t *testing.T) {
	git := hotfixTestRepo(t)
	git("commit", "--allow-empty", "-m", "Fix")
	commit := git("rev-parse", "HEAD")
	git("tag", "1.0.1", "HEAD^")
	if err := hotfixPushTag("1.0.1", commit); err == nil || !strings.Contains(err.Error(), "local tag") {
		t.Fatalf("expected local mismatch, got %v", err)
	}
	git("push", "origin", "refs/tags/1.0.1")
	git("tag", "-d", "1.0.1")
	if err := hotfixPushTag("1.0.1", commit); err == nil || !strings.Contains(err.Error(), "remote tag") {
		t.Fatalf("expected remote mismatch, got %v", err)
	}
	if git("tag", "--list", "1.0.1") != "" {
		t.Fatal("remote mismatch must not create a local tag")
	}
}
