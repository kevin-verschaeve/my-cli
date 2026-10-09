package commands

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"mycli/app"

	"github.com/AlecAivazis/survey/v2"
	"github.com/symfony-cli/console"
	"github.com/symfony-cli/terminal"
)

var (
	hotfixVersionPattern   = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-.*)?$`)
	hotfixCandidatePattern = regexp.MustCompile(`^(\d+\.\d+\.\d+)(-.*)?$`)
)

func hotfixAsk(message string) string {
	return terminal.AskString(message+" ", func(s string) (string, bool) { return s, true })
}

func nextPatchVersion(tag string) (string, error) {
	m := hotfixVersionPattern.FindStringSubmatch(strings.TrimSpace(tag))
	if m == nil {
		return "", fmt.Errorf("tag %q is not a valid semantic version", tag)
	}

	patch, err := strconv.Atoi(m[3])
	if err != nil {
		return "", fmt.Errorf("invalid patch version in tag %q: %w", tag, err)
	}

	return fmt.Sprintf("%s.%s.%d", m[1], m[2], patch+1), nil
}

func hotfixTagName(branchName, prefix string) (string, error) {
	trimmedBranch := strings.TrimSpace(branchName)
	if trimmedBranch == "" {
		return "", fmt.Errorf("branch name is required")
	}

	tagName := strings.TrimSpace(strings.TrimPrefix(trimmedBranch, prefix+"/"))
	if tagName == "" {
		return "", fmt.Errorf("tag name is required")
	}

	if hotfixVersionPattern.MatchString(tagName) {
		return tagName, nil
	}

	return "", fmt.Errorf("tag %q is not a valid semantic version", tagName)
}

// hotfixTagCandidates expects tags sorted by descending Git version order.
// It keeps the latest version's tags, placing the plain release first.
func hotfixTagCandidates(allTags string) []string {
	base := ""
	var candidates []string
	for _, tag := range strings.Split(strings.TrimSpace(allTags), "\n") {
		m := hotfixCandidatePattern.FindStringSubmatch(tag)
		if m == nil {
			continue
		}
		if base == "" {
			base = m[1]
		}
		if m[1] != base {
			break
		}
		candidates = append(candidates, tag)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i] == base && candidates[j] != base
	})
	return candidates
}

func hotfixStart(prefix string) (string, error) {
	allTags, err := app.RunGitCommand("tag", "-l", "--sort=-v:refname")
	if err != nil {
		return "", fmt.Errorf("unable to list tags: %w", err)
	}
	candidates := hotfixTagCandidates(allTags)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no tag matching X.Y.Z(-*)? pattern found")
	}

	options := append(candidates, "Other (I will provide it)")
	choice := 0
	err = survey.AskOne(&survey.Select{
		Message: "Select the tag you want to start from:",
		Options: options,
	}, &choice)
	if err != nil {
		return "", err
	}
	var latestTag string
	if choice < len(candidates) {
		latestTag = candidates[choice]
	} else {
		latestTag = strings.TrimSpace(hotfixAsk("Provide the tag to use:"))
	}
	if latestTag == "" {
		return "", fmt.Errorf("tag is required")
	}
	if _, err := app.RunGitCommand("rev-parse", "--verify", "refs/tags/"+latestTag+"^{commit}"); err != nil {
		return "", fmt.Errorf("release tag %q does not exist or does not reference a commit", latestTag)
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note(fmt.Sprintf("Using tag: %s", latestTag))

	nextVersion, err := nextPatchVersion(latestTag)
	if err != nil {
		return "", fmt.Errorf("unable to compute hotfix version from tag %q: %w", latestTag, err)
	}
	branchName := fmt.Sprintf("%s/%s", prefix, nextVersion)
	if !terminal.AskConfirmation(fmt.Sprintf("Create hotfix from tag %s and branch %s?", latestTag, branchName), true) {
		name := strings.TrimSpace(hotfixAsk("Hotfix branch name (without prefix):"))
		if name == "" {
			return "", fmt.Errorf("branch name is required")
		}
		branchName = fmt.Sprintf("%s/%s", prefix, name)
		if err := hotfixValidateNewBranch(branchName, prefix); err != nil {
			return "", err
		}
		if !terminal.AskConfirmation(fmt.Sprintf("Create hotfix from tag %s and branch %s?", latestTag, branchName), true) {
			return "", fmt.Errorf("hotfix creation cancelled by user")
		}
	}
	if err := hotfixValidateNewBranch(branchName, prefix); err != nil {
		return "", err
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note(fmt.Sprintf("Creating branch: %s", branchName))

	// Create branch from tag
	_, err = app.RunGitCommand("checkout", "-b", branchName, "refs/tags/"+latestTag)
	if err != nil {
		return "", fmt.Errorf("unable to create branch: %w", err)
	}
	return branchName, nil
}

func hotfixEnsureClean() error {
	for _, operation := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		path, err := app.RunGitCommand("rev-parse", "--git-path", operation)
		if err != nil {
			return fmt.Errorf("not a Git repository: %w", err)
		}
		if _, err := os.Stat(strings.TrimSpace(path)); err == nil {
			return fmt.Errorf("Git operation %s is in progress; resolve or abort it before continuing", operation)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	status, err := app.RunGitCommand("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("working tree is not clean; commit or stash your changes (including untracked files) before continuing")
	}
	return nil
}

func hotfixPreflight() error {
	if err := hotfixEnsureClean(); err != nil {
		return err
	}
	if _, err := app.RunGitCommand("remote", "get-url", "origin"); err != nil {
		return fmt.Errorf("remote 'origin' is required: %w", err)
	}
	if _, err := app.RunGitCommand("fetch", "--prune", "--tags", "origin"); err != nil {
		return fmt.Errorf("unable to refresh remote branches and tags: %w", err)
	}
	return nil
}

func hotfixValidateNewBranch(branch, prefix string) error {
	tag, err := hotfixTagName(branch, prefix)
	if err != nil {
		return err
	}
	if _, err := app.RunGitCommand("check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch, "refs/tags/" + tag} {
		if _, err := app.RunGitCommand("rev-parse", "--verify", ref); err == nil {
			return fmt.Errorf("%s already exists; resume the existing hotfix or choose another version", ref)
		}
	}
	return nil
}

func hotfixPushTag(tagName string) error {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	existingTag, err := app.RunGitCommand("tag", "--list", tagName)
	if err != nil {
		return fmt.Errorf("unable to check tag %q: %w", tagName, err)
	}
	if strings.TrimSpace(existingTag) == "" {
		if _, err := app.RunGitCommand("tag", tagName); err != nil {
			return fmt.Errorf("unable to create tag %q: %w", tagName, err)
		}
		ui.Note(fmt.Sprintf("Created tag: %s", tagName))
	}

	if _, err := app.RunGitCommand("push", "origin", tagName); err != nil {
		return fmt.Errorf("unable to push tag %q: %w", tagName, err)
	}
	ui.Success(fmt.Sprintf("Tag %s pushed", tagName))
	return nil
}

func hotfixBackport(branchName, target string) error {
	for _, args := range [][]string{
		{"checkout", target},
		{"pull", "origin", target},
		{"merge", "--no-ff", branchName},
		{"push", "origin", target},
	} {
		if _, err := app.RunGitCommand(args...); err != nil {
			return fmt.Errorf("backport to %s failed on 'git %s': %w", target, strings.Join(args, " "), err)
		}
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Success(fmt.Sprintf("Backported to %s", target))
	return nil
}

func hotfixPublish(branchName string) error {
	if _, err := app.RunGitCommand("push", "-u", "origin", branchName); err != nil {
		return fmt.Errorf("unable to push branch: %w", err)
	}
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	ui.Success("Branch pushed")
	ui.Note("Request a review, then run 'mycli hotfix finish' when it is approved.")
	return nil
}

func hotfixFinish(branchName, prefix string) error {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	if !terminal.AskConfirmation("Review completed and verified?", false) {
		ui.Note("Finalization postponed. Run 'mycli hotfix finish' after review.")
		return nil
	}
	tagName, err := hotfixTagName(branchName, prefix)
	if err != nil {
		return fmt.Errorf("unable to determine tag name for branch %q: %w", branchName, err)
	}
	if terminal.AskConfirmation(fmt.Sprintf("Push the release tag %s?", tagName), false) {
		if err := hotfixPushTag(tagName); err != nil {
			return err
		}
	}
	if terminal.AskConfirmation("Backport to develop?", true) {
		if err := hotfixBackport(branchName, "develop"); err != nil {
			return err
		}
	}
	if terminal.AskConfirmation("Backport to main/master?", true) {
		targetBranch := "main"
		if _, err := app.RunGitCommand("rev-parse", "--verify", "origin/main"); err != nil {
			targetBranch = "master"
		}
		if err := hotfixBackport(branchName, targetBranch); err != nil {
			return err
		}
	}
	ui.Success("Hotfix workflow completed")
	return nil
}

func hotfixActions(current, prefix string) []string {
	if strings.HasPrefix(current, prefix+"/") {
		return []string{"publish", "finish", "start"}
	}
	return []string{"start"}
}

var Hotfix = &console.Command{
	Name:  "hotfix",
	Usage: "Hotfix workflow: start, publish, or finish; omit the action for an interactive menu",
	Args: console.ArgDefinition{
		{Name: "action", Optional: true, Description: "start, publish, or finish"},
	},
	Action: func(c *console.Context) error {
		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		config, err := app.LoadConfig()
		if err != nil {
			return fmt.Errorf("unable to load configuration: %w", err)
		}
		prefix := config.HotfixPrefix
		if prefix == "" {
			prefix = "hotfix"
		}

		current, err := app.RunGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return fmt.Errorf("unable to read current branch: %w", err)
		}
		current = strings.TrimSpace(current)
		action := c.Args().Get("action")
		if action == "" {
			options := hotfixActions(current, prefix)
			if err := survey.AskOne(&survey.Select{Message: "Next hotfix step:", Options: options}, &action); err != nil {
				return err
			}
		}
		if action != "start" && action != "publish" && action != "finish" {
			return fmt.Errorf("unknown hotfix action %q: use start, publish, or finish", action)
		}
		if err := hotfixPreflight(); err != nil {
			return err
		}
		switch action {
		case "start":
			branch, err := hotfixStart(prefix)
			if err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("Branch %s created. Implement and commit your fix.", branch))
			ui.Note("When ready, run 'mycli hotfix publish'.")
			return nil
		case "publish", "finish":
			if !strings.HasPrefix(current, prefix+"/") {
				return fmt.Errorf("check out a %s/* branch before running 'mycli hotfix %s'", prefix, action)
			}
			if action == "publish" {
				return hotfixPublish(current)
			}
			return hotfixFinish(current, prefix)
		default:
			return fmt.Errorf("unknown hotfix action %q: use start, publish, or finish", action)
		}
	},
}
