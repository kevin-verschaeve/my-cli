package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestHotfixTagCandidates(t *testing.T) {
	for _, tt := range []struct {
		name string
		tags string
		want []string
	}{
		{name: "empty"},
		{name: "unmatched", tags: "v2.0.0\nrelease"},
		{name: "release first", tags: "1.2.3-rc2\n1.2.3-rc1\n1.2.3\n1.2.2", want: []string{"1.2.3", "1.2.3-rc2", "1.2.3-rc1", "1.2.2"}},
		{name: "prereleases only", tags: "1.2.3-rc2\n1.2.3-rc1\n1.2.2", want: []string{"1.2.3-rc2", "1.2.3-rc1", "1.2.2"}},
		{name: "skip unmatched tags", tags: "v9.0.0\n1.2.3-rc2\nignored\n1.2.3\n1.2.2", want: []string{"1.2.3", "1.2.3-rc2", "1.2.2"}},
		{name: "trim outer whitespace", tags: " \n1.2.3\n1.2.2\n ", want: []string{"1.2.3", "1.2.2"}},
		{name: "limit version families", tags: "1.2.6\n1.2.5\n1.2.4\n1.2.3\n1.2.2\n1.2.1", want: []string{"1.2.6", "1.2.5", "1.2.4", "1.2.3", "1.2.2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := hotfixTagCandidates(tt.tags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("hotfixTagCandidates(%q) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestHotfixGitSteps(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for this integration test")
	}

	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	remote := filepath.Join(root, "remote.git")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--bare", remote)
	git("init", repo)
	git("-C", repo, "config", "user.name", "Hotfix Test")
	git("-C", repo, "config", "user.email", "hotfix@example.test")
	git("-C", repo, "config", "commit.gpgsign", "false")
	git("-C", repo, "config", "tag.gpgsign", "false")
	git("-C", repo, "remote", "add", "origin", remote)
	git("-C", repo, "checkout", "-b", "develop")
	git("-C", repo, "commit", "--allow-empty", "-m", "Initial release")
	git("-C", repo, "branch", "main")
	git("-C", repo, "checkout", "-b", "hotfix/1.0.1")
	git("-C", repo, "commit", "--allow-empty", "-m", "Hotfix")
	git("-C", repo, "push", "origin", "develop", "main", "hotfix/1.0.1")
	hotfixCommit := git("-C", repo, "rev-parse", "HEAD")

	// The production helpers use the process working directory; do not run in parallel.
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	for i := 0; i < 2; i++ {
		if err := hotfixPushTag("1.0.1"); err != nil {
			t.Fatalf("hotfixPushTag attempt %d: %v", i+1, err)
		}
	}
	if got := git("--git-dir", remote, "rev-parse", "refs/tags/1.0.1"); got != hotfixCommit {
		t.Fatalf("remote release tag = %s, want %s", got, hotfixCommit)
	}
	for _, target := range []string{"develop", "main"} {
		if err := hotfixBackport("hotfix/1.0.1", target); err != nil {
			t.Fatalf("hotfixBackport to %s: %v", target, err)
		}
		if got := git("-C", repo, "rev-parse", "HEAD^2"); got != hotfixCommit {
			t.Fatalf("%s merge second parent = %s, want %s", target, got, hotfixCommit)
		}
		local := git("-C", repo, "rev-parse", "HEAD")
		if got := git("--git-dir", remote, "rev-parse", "refs/heads/"+target); got != local {
			t.Fatalf("remote %s = %s, want %s", target, got, local)
		}
	}
}

func TestNextPatchVersion(t *testing.T) {
	for _, tt := range []struct {
		name string
		tag  string
		want string
	}{
		{name: "simple release", tag: "1.0.6", want: "1.0.7"},
		{name: "release candidate", tag: "1.0.6-rc1", want: "1.0.7"},
		{name: "whitespace", tag: " \t1.0.6-rc1\n", want: "1.0.7"},
		{name: "leading zeroes", tag: "01.02.006", want: "01.02.7"},
		{name: "empty suffix", tag: "1.0.6-", want: "1.0.7"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextPatchVersion(tt.tag)
			if err != nil {
				t.Fatalf("nextPatchVersion(%q) returned error: %v", tt.tag, err)
			}
			if got != tt.want {
				t.Fatalf("nextPatchVersion(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestHotfixTagName(t *testing.T) {
	for _, tt := range []struct {
		name   string
		branch string
		prefix string
		want   string
	}{
		{name: "hotfix branch", branch: "hotfix/1.0.7", prefix: "hotfix", want: "1.0.7"},
		{name: "custom prefix", branch: "fix/1.0.7-rc1", prefix: "fix", want: "1.0.7-rc1"},
		{name: "bare tag", branch: "1.0.7", prefix: "hotfix", want: "1.0.7"},
		{name: "whitespace", branch: " \thotfix/ 1.0.7 \n", prefix: "hotfix", want: "1.0.7"},
		{name: "empty suffix", branch: "hotfix/1.0.7-", prefix: "hotfix", want: "1.0.7-"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hotfixTagName(tt.branch, tt.prefix)
			if err != nil {
				t.Fatalf("hotfixTagName(%q, %q) returned error: %v", tt.branch, tt.prefix, err)
			}
			if got != tt.want {
				t.Fatalf("hotfixTagName(%q, %q) = %q, want %q", tt.branch, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestNextPatchVersionErrors(t *testing.T) {
	for _, tag := range []string{"", "1.2", "release/1.2.3", "1.2.3+build", "1.2.3\n-rc1", "v1.0.6", " \tv1.0.6-rc1\n"} {
		t.Run(tag, func(t *testing.T) {
			if _, err := nextPatchVersion(tag); err == nil {
				t.Fatalf("nextPatchVersion(%q) should fail", tag)
			}
		})
	}

	tag := "1.2." + strings.Repeat("9", strconv.IntSize)
	if _, err := nextPatchVersion(tag); err == nil || !strings.Contains(err.Error(), "invalid patch version") {
		t.Fatalf("nextPatchVersion(%q) should report an invalid patch version, got %v", tag, err)
	}
}

func TestHotfixTagNameErrors(t *testing.T) {
	for _, tt := range []struct {
		branch string
		want   string
	}{
		{branch: " \t", want: "branch name is required"},
		{branch: "hotfix/", want: "tag name is required"},
		{branch: "hotfix/ \t", want: "tag name is required"},
		{branch: "fix/1.0.7", want: `tag "fix/1.0.7" is not a valid semantic version`},
		{branch: "hotfix/custom", want: `tag "custom" is not a valid semantic version`},
		{branch: "hotfix/1.0.7+build", want: `tag "1.0.7+build" is not a valid semantic version`},
		{branch: "hotfix/v1.0.7", want: `tag "v1.0.7" is not a valid semantic version`},
		{branch: "v1.0.7-rc1", want: `tag "v1.0.7-rc1" is not a valid semantic version`},
	} {
		t.Run(tt.branch, func(t *testing.T) {
			_, err := hotfixTagName(tt.branch, "hotfix")
			if err == nil || err.Error() != tt.want {
				t.Fatalf("hotfixTagName(%q) error = %v, want %q", tt.branch, err, tt.want)
			}
		})
	}
}
