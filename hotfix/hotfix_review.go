package hotfix

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"mycli/app"

	"github.com/symfony-cli/terminal"
)

func hotfixVCSTool(vcs string) string {
	switch vcs {
	case "github":
		return "gh"
	case "gitlab":
		return "glab"
	default:
		return ""
	}
}

func hotfixRunVCS(tool string, args ...string) (string, error) {
	output, err := exec.Command(tool, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s failed: %w\n%s", tool, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

var hotfixReviewNotFound = errors.New("no matching review found")

type hotfixGitLabMR struct {
	IID    int    `json:"iid"`
	Target string `json:"target_branch"`
	State  string `json:"state"`
}

func hotfixSelectGitLabMR(output, target string) (int, error) {
	var mrs []hotfixGitLabMR
	if err := json.Unmarshal([]byte(output), &mrs); err != nil {
		return 0, fmt.Errorf("invalid GitLab MR list: %w", err)
	}
	for _, mr := range mrs {
		if mr.IID > 0 && mr.State == "opened" && (target == "" || mr.Target == target) {
			return mr.IID, nil
		}
	}
	return 0, hotfixReviewNotFound
}

func hotfixGitLabMRDetails(branch, target string) (string, int, error) {
	endpoint := "projects/:id/merge_requests?scope=all&state=all&per_page=100&source_branch=" + url.QueryEscape(branch)
	if target != "" {
		endpoint += "&target_branch=" + url.QueryEscape(target)
	}
	output, err := hotfixRunVCS("glab", "api", endpoint)
	if err != nil {
		return "", 0, err
	}
	iid, err := hotfixSelectGitLabMR(output, target)
	if err != nil {
		return "", 0, err
	}
	details, err := hotfixRunVCS("glab", "api", fmt.Sprintf("projects/:id/merge_requests/%d", iid))
	return details, iid, err
}

func hotfixReviewTarget(configured string) string {
	if configured != "" {
		return configured
	}
	for _, branch := range []string{"main", "master", "develop"} {
		if _, err := app.RunGitCommand("rev-parse", "--verify", "refs/remotes/origin/"+branch); err == nil {
			return branch
		}
	}
	return ""
}

// hotfixOpenReview reuses an existing PR/MR before offering to create one.
func hotfixOpenReview(vcs, branch, target string) error {
	tool := hotfixVCSTool(vcs)
	if tool == "" || app.CheckCommandExists(tool) != nil {
		return fmt.Errorf("install gh (GitHub) or glab (GitLab), and configure vcs to open reviews")
	}
	if vcs == "github" {
		output, err := hotfixRunVCS(tool, "pr", "list", "--head", branch, "--base", target, "--state", "open", "--json", "url")
		if err != nil {
			return err
		}
		var prs []struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(output), &prs); err != nil {
			return fmt.Errorf("invalid GitHub PR list: %w", err)
		}
		if len(prs) > 0 {
			terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note("Review: " + prs[0].URL)
			_, err = hotfixRunVCS(tool, "pr", "view", prs[0].URL, "--web")
			return err
		}
		output, err = hotfixRunVCS(tool, "pr", "create", "--head", branch, "--base", target, "--title", "Hotfix "+branch, "--body", "Hotfix for review and release.")
		if err != nil {
			return err
		}
		terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note("Review: " + output)
		_, err = hotfixRunVCS(tool, "pr", "view", branch, "--web")
		return err
	}
	output, iid, err := hotfixGitLabMRDetails(branch, target)
	if err == nil {
		var mr struct {
			URL string `json:"web_url"`
		}
		if err := json.Unmarshal([]byte(output), &mr); err != nil {
			return err
		}
		terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note("Review: " + mr.URL)
		_, err = hotfixRunVCS(tool, "mr", "view", fmt.Sprint(iid), "--web")
		return err
	}
	if !errors.Is(err, hotfixReviewNotFound) {
		return err
	}
	output, err = hotfixRunVCS(tool, "mr", "create", "--source-branch", branch, "--target-branch", target, "--title", "Hotfix "+branch, "--description", "Hotfix for review and release.", "--yes")
	if err == nil {
		terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin).Note(output)
		_, err = hotfixRunVCS(tool, "mr", "view", branch, "--web")
	}
	return err
}
