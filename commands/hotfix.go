package commands

import (
	"fmt"
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
		if !terminal.AskConfirmation(fmt.Sprintf("Create hotfix from tag %s and branch %s?", latestTag, branchName), true) {
			return "", fmt.Errorf("hotfix creation cancelled by user")
		}
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note(fmt.Sprintf("Creating branch: %s", branchName))

	// Create branch from tag
	_, err = app.RunGitCommand("checkout", "-b", branchName, latestTag)
	if err != nil {
		return "", fmt.Errorf("unable to create branch: %w", err)
	}
	return branchName, nil
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

var Hotfix = &console.Command{
	Name:  "hotfix",
	Usage: "Interactive hotfix workflow: create branch from latest tag, push, and manage backports",
	Action: func(c *console.Context) error {
		ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
		prefix := app.GetConfig("HotfixPrefix")
		if prefix == "" {
			prefix = "hotfix"
		}

		var branchName string
		var err error
		// Preserve the fallback to starting a hotfix if the current branch cannot be read.
		current, _ := app.RunGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		current = strings.TrimSpace(current)
		if strings.HasPrefix(current, prefix+"/") && terminal.AskConfirmation(fmt.Sprintf("Resume hotfix on branch %s?", current), true) {
			branchName = current
			ui.Note(fmt.Sprintf("Resuming on %s", branchName))
		} else if branchName, err = hotfixStart(prefix); err != nil {
			return err
		}

		// Wait for dev confirmation
		if !terminal.AskConfirmation("Have you finished the fix? Ready to push?", false) {
			ui.Note("Hotfix cancelled")
			return nil
		}

		// Push branch
		_, err = app.RunGitCommand("push", "-u", "origin", branchName)
		if err != nil {
			return fmt.Errorf("unable to push branch: %w", err)
		}
		ui.Success("Branch pushed")

		// Wait for review
		ui.Note("Waiting for review...")
		if !terminal.AskConfirmation("Review completed and verified?", false) {
			ui.Note("Skipping backport")
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
	},
}
