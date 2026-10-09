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

// hotfixTagCandidates expects descending Git version order and keeps the five
// latest version families, placing plain releases before their prereleases.
func hotfixTagCandidates(allTags string) []string {
	bases := make(map[string]int)
	var candidates []string
	for _, tag := range strings.Split(strings.TrimSpace(allTags), "\n") {
		m := hotfixCandidatePattern.FindStringSubmatch(tag)
		if m == nil {
			continue
		}
		if _, found := bases[m[1]]; !found {
			if len(bases) == 5 {
				continue
			}
			bases[m[1]] = len(bases)
		}
		candidates = append(candidates, tag)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a := hotfixCandidatePattern.FindStringSubmatch(candidates[i])[1]
		b := hotfixCandidatePattern.FindStringSubmatch(candidates[j])[1]
		if a != b {
			return bases[a] < bases[b]
		}
		return candidates[i] == a && candidates[j] != b
	})
	return candidates
}

func hotfixReleaseOptions(candidates []string, dates map[string]string, production string) ([]string, int) {
	options := make([]string, 0, len(candidates)+1)
	defaultChoice := 0
	stableFound := false
	for i, tag := range candidates {
		label := tag
		if date := dates[tag]; date != "" {
			label += " — " + date
		}
		if hotfixCandidatePattern.FindStringSubmatch(tag)[2] != "" {
			label += " [prerelease]"
		} else if !stableFound {
			defaultChoice, stableFound = i, true
		}
		if tag == production {
			label += " [configured production version]"
		}
		options = append(options, label)
	}
	if production != "" {
		for i, tag := range candidates {
			if tag == production {
				defaultChoice = i
			}
		}
	}
	return append(options, "Other (I will provide it)"), defaultChoice
}

func hotfixStart(prefix, production string, store *hotfixStore) (string, error) {
	allTags, err := app.RunGitCommand("tag", "-l", "--sort=-v:refname")
	if err != nil {
		return "", fmt.Errorf("unable to list tags: %w", err)
	}
	candidates := hotfixTagCandidates(allTags)
	if production != "" {
		if !hotfixVersionPattern.MatchString(production) {
			return "", fmt.Errorf("configured production tag %q is not a valid release version", production)
		}
		if _, err := app.RunGitCommand("rev-parse", "--verify", "refs/tags/"+production+"^{commit}"); err != nil {
			return "", fmt.Errorf("configured production tag %q was not found on origin; update hotfix_production_tag", production)
		}
		found := false
		for _, tag := range candidates {
			found = found || tag == production
		}
		if !found {
			candidates = append([]string{production}, candidates...)
		}
	}
	datesOutput, err := app.RunGitCommand("for-each-ref", "--format=%(refname:strip=2)\t%(creatordate:short)", "refs/tags")
	if err != nil {
		return "", fmt.Errorf("unable to read tag dates: %w", err)
	}
	dates := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(datesOutput), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			dates[parts[0]] = parts[1]
		}
	}
	options, defaultChoice := hotfixReleaseOptions(candidates, dates, production)
	choice := 0
	err = survey.AskOne(&survey.Select{
		Message: "Which deployed release needs the fix? (highest tag is not necessarily production)",
		Options: options,
		Default: options[defaultChoice],
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
	baseCommit, err := app.RunGitCommand("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	store.Workflows[branchName] = &hotfixState{
		Branch: branchName, Base: latestTag, BaseCommit: strings.TrimSpace(baseCommit), Phase: "started",
	}
	if err := store.save(); err != nil {
		return "", fmt.Errorf("branch %s was created, but checkpoint could not be saved: %w", branchName, err)
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

func hotfixRemoteTagCommit(output, tagName string) string {
	commit := ""
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] == "refs/tags/"+tagName+"^{}" {
			return fields[0]
		}
		if fields[1] == "refs/tags/"+tagName {
			commit = fields[0]
		}
	}
	return commit
}

func hotfixPublishedCommit(branchName string) (string, error) {
	commit, err := app.RunGitCommand("rev-parse", "--verify", "refs/heads/"+branchName+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("unable to resolve hotfix branch: %w", err)
	}
	commit = strings.TrimSpace(commit)
	remote, err := app.RunGitCommand("ls-remote", "--exit-code", "origin", "refs/heads/"+branchName)
	if err != nil {
		return "", fmt.Errorf("hotfix is not published or origin is unavailable; run 'mycli hotfix publish': %w", err)
	}
	fields := strings.Fields(remote)
	if len(fields) != 2 || fields[0] != commit {
		return "", fmt.Errorf("local and published hotfix commits differ; synchronize and run 'mycli hotfix publish' before review")
	}
	return commit, nil
}

func hotfixPushTag(tagName, commit string) error {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	if !hotfixVersionPattern.MatchString(tagName) {
		return fmt.Errorf("invalid release tag %q", tagName)
	}
	resolved, err := app.RunGitCommand("rev-parse", "--verify", commit+"^{commit}")
	if err != nil {
		return fmt.Errorf("unable to resolve release commit: %w", err)
	}
	commit = strings.TrimSpace(resolved)
	remote, err := app.RunGitCommand("ls-remote", "origin", "refs/tags/"+tagName, "refs/tags/"+tagName+"^{}")
	if err != nil {
		return fmt.Errorf("unable to check remote tag: %w", err)
	}
	if remoteCommit := hotfixRemoteTagCommit(remote, tagName); remoteCommit != "" && remoteCommit != commit {
		return fmt.Errorf("remote tag %s references %s instead of validated commit %s; it will not be overwritten", tagName, remoteCommit, commit)
	}
	existingTag, err := app.RunGitCommand("tag", "--list", tagName)
	if err != nil {
		return fmt.Errorf("unable to check tag %q: %w", tagName, err)
	}
	remoteExists := hotfixRemoteTagCommit(remote, tagName) != ""
	if strings.TrimSpace(existingTag) == "" && remoteExists {
		if _, err := app.RunGitCommand("fetch", "origin", "refs/tags/"+tagName+":refs/tags/"+tagName); err != nil {
			return fmt.Errorf("unable to recover existing release tag: %w", err)
		}
		existingTag = tagName
	}
	if strings.TrimSpace(existingTag) == "" {
		if _, err := app.RunGitCommand("tag", "-a", tagName, "-m", "Hotfix release "+tagName, commit); err != nil {
			return fmt.Errorf("unable to create tag %q: %w", tagName, err)
		}
		ui.Note(fmt.Sprintf("Created tag: %s", tagName))
	} else {
		existingCommit, err := app.RunGitCommand("rev-parse", "--verify", "refs/tags/"+tagName+"^{commit}")
		if err != nil {
			return err
		}
		if strings.TrimSpace(existingCommit) != commit {
			return fmt.Errorf("local tag %s does not reference validated commit %s; it will not be overwritten", tagName, commit)
		}
	}

	if remoteExists {
		ui.Success(fmt.Sprintf("Tag %s already published at validated commit %s", tagName, commit))
		return nil
	}
	if _, err := app.RunGitCommand("push", "origin", "refs/tags/"+tagName); err != nil {
		return fmt.Errorf("unable to push tag %q: %w", tagName, err)
	}
	ui.Success(fmt.Sprintf("Tag %s pushed", tagName))
	return nil
}

func hotfixPublish(store *hotfixStore, state *hotfixState, vcs, reviewTarget string) error {
	branchName := state.Branch
	if state.Phase == "finishing" || state.Phase == "completed" {
		return fmt.Errorf("release commit is frozen; resume finalization or start a new hotfix instead of publishing more commits")
	}
	if err := hotfixFindBase(state); err != nil {
		return err
	}
	commit, err := app.RunGitCommand("rev-parse", "--verify", "refs/heads/"+branchName+"^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(commit) == state.BaseCommit {
		return fmt.Errorf("no fix commit found; implement and commit your fix before publishing")
	}
	if _, err := app.RunGitCommand("push", "-u", "origin", branchName); err != nil {
		return fmt.Errorf("unable to push branch: %w", err)
	}
	state.Commit, state.Phase = strings.TrimSpace(commit), "published"
	if err := store.save(); err != nil {
		return err
	}
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	ui.Success("Branch pushed")
	hotfixReviewLinks(branchName, vcs, reviewTarget)
	ui.Note("Request a review, then run 'mycli hotfix finish' when it is approved.")
	return nil
}

func hotfixFinish(store *hotfixStore, state *hotfixState, prefix string, config *app.Config) error {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	branchName := state.Branch
	if state.Phase == "completed" {
		ui.Success("Hotfix already finalized. Review backports may still be awaiting merge.")
		return nil
	}
	commit, err := hotfixPublishedCommit(branchName)
	if err != nil {
		return err
	}
	if state.Phase == "finishing" && state.Commit != commit {
		return fmt.Errorf("hotfix changed after release validation; expected %s, found %s", state.Commit, commit)
	}
	if err := hotfixFindBase(state); err != nil {
		return err
	}
	if commit == state.BaseCommit {
		return fmt.Errorf("no fix commit found; nothing to finalize")
	}
	ui.Note(fmt.Sprintf("Release commit: %s (%s)", commit, branchName))
	if state.Phase != "finishing" {
		if err := hotfixVerifyReview(config.VersionControlService, branchName, commit); err != nil {
			return err
		}
		if !terminal.AskConfirmation("Finalize this validated hotfix commit?", false) {
			ui.Note("Finalization postponed. Run 'mycli hotfix finish' after review.")
			return nil
		}
		state.Commit, state.Phase = commit, "finishing"
		if err := store.save(); err != nil {
			return err
		}
	}
	tagName, err := hotfixTagName(branchName, prefix)
	if err != nil {
		return fmt.Errorf("unable to determine tag name for branch %q: %w", branchName, err)
	}
	state.Tag = tagName
	if state.TagStatus == "" {
		state.TagStatus = "skipped"
		if terminal.AskConfirmation(fmt.Sprintf("Push release tag %s at validated commit %s?", tagName, commit), false) {
			state.TagStatus = "pending"
		}
		if err := store.save(); err != nil {
			return err
		}
	}
	if state.TagStatus == "pending" {
		if err := hotfixPushTag(tagName, commit); err != nil {
			return err
		}
		state.TagStatus = "published"
		if err := store.save(); err != nil {
			return err
		}
	}
	if err := hotfixPlanBackports(store, state, config); err != nil {
		return err
	}
	for i := range state.Backports {
		if err := hotfixRunBackport(store, state, i, config.VersionControlService); err != nil {
			return err
		}
	}
	state.Phase = "completed"
	if err := store.save(); err != nil {
		return err
	}
	ui.Success("Hotfix finalization completed; review backports still need approval and merge.")
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
	Usage: "Hotfix workflow: start, publish, finish, or resume; omit the action for an interactive menu",
	Args: console.ArgDefinition{
		{Name: "action", Optional: true, Description: "start, publish, finish, or resume"},
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
		store, err := hotfixLoadStore()
		if err != nil {
			return err
		}
		var resumable []string
		for branch, state := range store.Workflows {
			if state.Phase != "completed" {
				resumable = append(resumable, branch)
			}
		}
		sort.Strings(resumable)

		current, err := app.RunGitCommand("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return fmt.Errorf("unable to read current branch: %w", err)
		}
		current = strings.TrimSpace(current)
		action := c.Args().Get("action")
		if action == "" {
			options := hotfixActions(current, prefix)
			defaultAction := options[0]
			if state := store.Workflows[current]; state != nil {
				defaultAction = hotfixDefaultAction(state)
			}
			if len(resumable) > 0 {
				options = append(options, "resume")
				if !strings.HasPrefix(current, prefix+"/") {
					defaultAction = "resume"
				}
			}
			if err := survey.AskOne(&survey.Select{Message: "Next hotfix step:", Options: options, Default: defaultAction}, &action); err != nil {
				return err
			}
		}
		if action != "start" && action != "publish" && action != "finish" && action != "resume" {
			return fmt.Errorf("unknown hotfix action %q: use start, publish, finish, or resume", action)
		}
		if err := hotfixPreflight(); err != nil {
			return err
		}
		if action == "resume" {
			if len(resumable) == 0 {
				return fmt.Errorf("no unfinished hotfix checkpoint found")
			}
			current = resumable[0]
			if len(resumable) > 1 {
				if err := survey.AskOne(&survey.Select{Message: "Hotfix to resume:", Options: resumable}, &current); err != nil {
					return err
				}
			}
			action = hotfixDefaultAction(store.Workflows[current])
			ui.Note(fmt.Sprintf("Resuming %s at %s", current, action))
		}
		switch action {
		case "start":
			branch, err := hotfixStart(prefix, strings.TrimSpace(config.HotfixProductionTag), store)
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
				if _, err := app.RunGitCommand("checkout", current); err != nil {
					return err
				}
				return hotfixPublish(store, store.state(current), config.VersionControlService, config.HotfixReviewTarget)
			}
			return hotfixFinish(store, store.state(current), prefix, config)
		default:
			return fmt.Errorf("unknown hotfix action %q: use start, publish, finish, or resume", action)
		}
	},
}
