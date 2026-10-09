package hotfix

import (
	"fmt"
	"strings"

	"mycli/app"

	"github.com/symfony-cli/terminal"
)

func hotfixSummary(state *hotfixState, current string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "Hotfix: %s\nStage: %s\n", state.Branch, state.Phase)
	if state.Base != "" {
		fmt.Fprintf(&out, "Starting release: %s\n", state.Base)
	}
	if state.Commit != "" {
		fmt.Fprintf(&out, "Published/validated commit: %s\n", state.Commit)
	} else {
		out.WriteString("Publication: not recorded yet\n")
	}
	tagStatus := state.TagStatus
	if tagStatus == "" {
		tagStatus = "not decided"
	}
	tag := state.Tag
	if tag == "" {
		branch := strings.TrimSpace(state.Branch)
		if separator := strings.LastIndex(branch, "/"); separator >= 0 {
			if inferred, err := hotfixTagName(branch, branch[:separator]); err == nil {
				tag = inferred
			}
		}
	}
	fmt.Fprintf(&out, "Release tag: %s (%s)\n", tag, tagStatus)
	if !state.BackportsPlanned {
		out.WriteString("Backports: not selected yet\n")
	} else if len(state.Backports) == 0 {
		out.WriteString("Backports: none selected\n")
	}
	for _, backport := range state.Backports {
		status := backport.Phase
		if status == "done" {
			if backport.Outcome == "already-present" {
				status = "fix already present; no review needed"
			} else if backport.Mode == "review" {
				status = "review opened; approval/merge still required"
			} else {
				status = "pushed to target"
			}
		}
		fmt.Fprintf(&out, "Backport %s [%s]: %s (%s)\n", backport.Target, backport.Mode, status, backport.Branch)
	}
	if state.LastError != "" {
		fmt.Fprintf(&out, "Last failure: %s\n", state.LastError)
	}
	fmt.Fprintf(&out, "Checked-out branch: %s\n", current)
	switch state.Phase {
	case "completed":
		out.WriteString("Next: merge any pending backport reviews. Start a new hotfix for further changes.")
	case "finishing":
		out.WriteString("Next: resolve any pending Git operation, then run 'exo hotfix resume'.")
	case "published":
		out.WriteString("Next: obtain approval and green CI, then run 'exo hotfix finish'.")
	default:
		out.WriteString("Next: implement and commit the fix, then run 'exo hotfix publish'.")
	}
	return out.String()
}

func hotfixShowSummary(state *hotfixState) {
	current, err := app.RunGitCommand("branch", "--show-current")
	if err != nil || strings.TrimSpace(current) == "" {
		current = "detached HEAD / unavailable"
	}
	terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note(hotfixSummary(state, strings.TrimSpace(current)))
}
