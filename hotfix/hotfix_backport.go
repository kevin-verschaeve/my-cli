package hotfix

import (
	"fmt"
	"strings"

	"mycli/app"

	"github.com/AlecAivazis/survey/v2"
	"github.com/symfony-cli/terminal"
)

func hotfixBackportTargets(configured []string) ([]string, error) {
	targets := configured
	if len(targets) == 0 {
		if _, err := app.RunGitCommand("rev-parse", "--verify", "refs/remotes/origin/develop"); err == nil {
			targets = append(targets, "develop")
		}
		if release := hotfixReviewTarget(""); release != "" {
			targets = append(targets, release)
		}
	}
	var result []string
	seen := make(map[string]bool)
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if seen[target] {
			continue
		}
		if target == "" {
			return nil, fmt.Errorf("backport target must not be empty")
		}
		if _, err := app.RunGitCommand("check-ref-format", "refs/heads/"+target); err != nil {
			return nil, fmt.Errorf("invalid backport target %q", target)
		}
		if _, err := app.RunGitCommand("rev-parse", "--verify", "refs/remotes/origin/"+target); err != nil {
			return nil, fmt.Errorf("backport target origin/%s was not found", target)
		}
		seen[target] = true
		result = append(result, target)
	}
	return result, nil
}

func hotfixPlanBackports(store *hotfixStore, state *hotfixState, config *app.Config) error {
	if state.BackportsPlanned {
		return nil
	}
	targets, err := hotfixBackportTargets(config.Hotfix.BackportTargets)
	if err != nil {
		return err
	}
	var selected []string
	if len(targets) > 0 {
		if err := survey.AskOne(&survey.MultiSelect{
			Message: "Select backport targets (space to select, enter to confirm):",
			Options: targets,
			Default: targets,
		}, &selected); err != nil {
			return err
		}
	}
	mode := config.Hotfix.BackportMode
	if mode == "" {
		mode = "review"
	}
	if mode != "review" && mode != "direct" {
		return fmt.Errorf("hotfix.backport_mode must be review or direct")
	}
	if len(selected) > 0 {
		if err := survey.AskOne(&survey.Select{
			Message: "Backport mode: review cherry-picks only the fix; direct merges and pushes to target branches",
			Options: []string{"review", "direct"},
			Default: mode,
		}, &mode); err != nil {
			return err
		}
		if mode == "review" {
			merges, err := app.RunGitCommand("rev-list", "--merges", state.BaseCommit+".."+state.Commit)
			if err != nil {
				return err
			}
			if strings.TrimSpace(merges) != "" {
				return fmt.Errorf("hotfix contains merge commits; select direct mode on resume or perform a manual backport; no backport plan was saved")
			}
			tool := hotfixVCSTool(config.VersionControlService)
			if tool == "" || app.CheckCommandExists(tool) != nil {
				return fmt.Errorf("review backports require authenticated gh/glab and a configured vcs; install it or explicitly select direct mode")
			}
		}
		if !terminal.AskConfirmation(fmt.Sprintf("Backport %s to [%s] using %s mode?", state.Commit, strings.Join(selected, ", "), mode), false) {
			return fmt.Errorf("backports postponed; run 'exo hotfix resume' to select them again")
		}
	}
	var planned []hotfixBackportState
	for _, target := range selected {
		branch := "backport/" + state.Branch + "/" + target
		if target == state.Branch {
			return fmt.Errorf("cannot backport a hotfix to itself")
		}
		if _, err := app.RunGitCommand("check-ref-format", "--branch", branch); err != nil {
			return err
		}
		for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
			if _, err := app.RunGitCommand("rev-parse", "--verify", ref); err == nil {
				return fmt.Errorf("backport branch %s already exists outside this checkpoint; inspect it before continuing", branch)
			}
		}
		planned = append(planned, hotfixBackportState{Target: target, Mode: mode, Branch: branch, Phase: "pending"})
	}
	state.Backports = planned
	state.BackportsPlanned = true
	return store.save()
}

func hotfixMissingPicks(state *hotfixState, branch, target string) ([]string, error) {
	merges, err := app.RunGitCommand("rev-list", "--merges", state.BaseCommit+".."+state.Commit)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merges) != "" {
		return nil, fmt.Errorf("hotfix contains merge commits; review backports require a linear fix history (use an explicit manual backport or direct mode)")
	}
	cherry, err := app.RunGitCommand("cherry", branch, state.Commit, state.BaseCommit)
	if err != nil {
		return nil, err
	}
	missing := make(map[string]bool)
	for _, line := range strings.Split(cherry, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "+" {
			missing[fields[1]] = true
		}
	}
	commits, err := app.RunGitCommand("rev-list", "--reverse", "--topo-order", state.BaseCommit+".."+state.Commit)
	if err != nil {
		return nil, err
	}
	log, err := app.RunGitCommand("log", "--format=%B", "refs/remotes/origin/"+target+".."+branch)
	if err != nil {
		return nil, err
	}
	var picks []string
	for _, commit := range strings.Fields(commits) {
		if missing[commit] && !strings.Contains(log, "(cherry picked from commit "+commit+")") {
			picks = append(picks, commit)
		}
	}
	return picks, nil
}

func hotfixRunBackport(store *hotfixStore, state *hotfixState, index int, vcs string) error {
	backport := &state.Backports[index]
	if backport.Phase == "done" {
		return nil
	}
	if err := hotfixEnsureClean(); err != nil {
		return err
	}
	if backport.Phase == "pending" {
		if _, err := app.RunGitCommand("checkout", "-b", backport.Branch, "refs/remotes/origin/"+backport.Target); err != nil {
			return err
		}
		backport.Phase = "applying"
		if err := store.save(); err != nil {
			return err
		}
	} else if _, err := app.RunGitCommand("checkout", backport.Branch); err != nil {
		return err
	}
	if backport.Phase == "applying" {
		var err error
		if backport.Mode == "review" {
			picks, e := hotfixMissingPicks(state, backport.Branch, backport.Target)
			if e != nil {
				return e
			}
			if len(picks) > 0 {
				_, err = app.RunGitCommand(append([]string{"cherry-pick", "-x"}, picks...)...)
			}
		} else {
			if _, e := app.RunGitCommand("merge-base", "--is-ancestor", state.Commit, "HEAD"); e != nil {
				_, err = app.RunGitCommand("merge", "--no-ff", "--no-edit", state.Commit)
			}
		}
		if err != nil {
			operation := "cherry-pick"
			if backport.Mode == "direct" {
				operation = "merge"
			}
			return fmt.Errorf("backport to %s stopped: %w\nResolve conflicts, stage changes, then run 'git %s --continue' and 'exo hotfix resume'. To abort the Git operation: 'git %s --abort'. Completed backports are preserved", backport.Target, err, operation, operation)
		}
		result, err := app.RunGitCommand("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		backport.ResultCommit = strings.TrimSpace(result)
		backport.Phase = "applied"
		if err := store.save(); err != nil {
			return err
		}
	}
	if backport.Phase == "applied" {
		current, err := app.RunGitCommand("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if strings.TrimSpace(current) != backport.ResultCommit {
			return fmt.Errorf("backport branch %s changed after preparation; inspect it before publishing", backport.Branch)
		}
		if backport.Mode == "review" {
			diff, err := app.RunGitCommand("diff", "refs/remotes/origin/"+backport.Target+"..."+backport.Branch)
			if err != nil {
				return err
			}
			if strings.TrimSpace(diff) == "" {
				backport.Phase = "done"
				backport.Outcome = "already-present"
				if err := store.save(); err != nil {
					return err
				}
				terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note("Fix already present on " + backport.Target + "; no empty review created.")
				return nil
			}
		}
		ref := "refs/heads/" + backport.Branch
		if backport.Mode == "direct" {
			ref = "refs/heads/" + backport.Target
		}
		if _, err := app.RunGitCommand("push", "origin", "refs/heads/"+backport.Branch+":"+ref); err != nil {
			return fmt.Errorf("unable to publish backport to %s (no force push was attempted): %w; run 'exo hotfix resume' to retry", backport.Target, err)
		}
		backport.Phase = "pushed"
		if err := store.save(); err != nil {
			return err
		}
	}
	if backport.Mode == "review" {
		if err := hotfixOpenReview(vcs, backport.Branch, backport.Target); err != nil {
			return fmt.Errorf("backport branch is pushed; opening its review failed: %w; run 'exo hotfix resume' to retry", err)
		}
	}
	backport.Phase = "done"
	if err := store.save(); err != nil {
		return err
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Success(fmt.Sprintf("Backport to %s: %s", backport.Target, backport.Mode))
	return nil
}
