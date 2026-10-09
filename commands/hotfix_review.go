package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
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

func hotfixSelectGitLabMR(output, target string, includeMerged bool) (int, error) {
	var mrs []hotfixGitLabMR
	if err := json.Unmarshal([]byte(output), &mrs); err != nil {
		return 0, fmt.Errorf("invalid GitLab MR list: %w", err)
	}
	for _, desired := range []string{"opened", "merged"} {
		if desired == "merged" && !includeMerged {
			break
		}
		for _, mr := range mrs {
			if mr.IID > 0 && mr.State == desired && (target == "" || mr.Target == target) {
				return mr.IID, nil
			}
		}
	}
	return 0, hotfixReviewNotFound
}

func hotfixGitLabMRDetails(branch, target string, includeMerged bool) (string, int, error) {
	endpoint := "projects/:id/merge_requests?scope=all&state=all&per_page=100&source_branch=" + url.QueryEscape(branch)
	if target != "" {
		endpoint += "&target_branch=" + url.QueryEscape(target)
	}
	output, err := hotfixRunVCS("glab", "api", endpoint)
	if err != nil {
		return "", 0, err
	}
	iid, err := hotfixSelectGitLabMR(output, target, includeMerged)
	if err != nil {
		return "", 0, err
	}
	details, err := hotfixRunVCS("glab", "api", fmt.Sprintf("projects/:id/merge_requests/%d", iid))
	return details, iid, err
}

func hotfixOpenURL(link string) error {
	parsed, err := url.Parse(link)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return fmt.Errorf("provider returned an invalid web URL")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", link)
	case "darwin":
		cmd = exec.Command("open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	default:
		return fmt.Errorf("browser opening is unsupported on this platform")
	}
	return cmd.Run()
}

type hotfixReview struct {
	URL      string
	Verified bool
}

func hotfixGitHubReview(output, commit string) (hotfixReview, error) {
	var pr struct {
		URL      string `json:"url"`
		State    string `json:"state"`
		Draft    bool   `json:"isDraft"`
		Head     string `json:"headRefOid"`
		Decision string `json:"reviewDecision"`
		Checks   []struct {
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			State      string `json:"state"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		return hotfixReview{}, fmt.Errorf("invalid GitHub review response: %w", err)
	}
	review := hotfixReview{URL: pr.URL}
	if pr.Head != commit || pr.Draft || (pr.State != "OPEN" && pr.State != "MERGED") {
		return review, fmt.Errorf("PR must be open or merged, not a draft, and reference the published hotfix commit")
	}
	if pr.Decision != "" && pr.Decision != "APPROVED" {
		return review, fmt.Errorf("PR review is %s; finalization is blocked", pr.Decision)
	}
	for _, check := range pr.Checks {
		if check.State != "" {
			if check.State != "SUCCESS" {
				return review, fmt.Errorf("PR status check is %s; finalization is blocked", check.State)
			}
		} else if check.Status != "COMPLETED" || (check.Conclusion != "SUCCESS" && check.Conclusion != "NEUTRAL" && check.Conclusion != "SKIPPED") {
			return review, fmt.Errorf("PR check is %s/%s; finalization is blocked", check.Status, check.Conclusion)
		}
	}
	review.Verified = pr.Decision == "APPROVED" && len(pr.Checks) > 0
	return review, nil
}

func hotfixGitLabReview(output, approvalsOutput, commit string) (hotfixReview, error) {
	var mr struct {
		URL      string `json:"web_url"`
		State    string `json:"state"`
		Draft    bool   `json:"draft"`
		WIP      bool   `json:"work_in_progress"`
		SHA      string `json:"sha"`
		Pipeline *struct {
			Status string `json:"status"`
			SHA    string `json:"sha"`
		} `json:"head_pipeline"`
	}
	if err := json.Unmarshal([]byte(output), &mr); err != nil {
		return hotfixReview{}, fmt.Errorf("invalid GitLab review response: %w", err)
	}
	review := hotfixReview{URL: mr.URL}
	if mr.SHA != commit || mr.Draft || mr.WIP || (mr.State != "opened" && mr.State != "merged") {
		return review, fmt.Errorf("MR must be open or merged, not a draft, and reference the published hotfix commit")
	}
	var approvals struct {
		Left     *int              `json:"approvals_left"`
		Approved []json.RawMessage `json:"approved_by"`
	}
	if err := json.Unmarshal([]byte(approvalsOutput), &approvals); err != nil {
		return review, fmt.Errorf("invalid GitLab approvals response: %w", err)
	}
	if approvals.Left != nil && *approvals.Left > 0 {
		return review, fmt.Errorf("MR still needs %d approval(s); finalization is blocked", *approvals.Left)
	}
	if mr.Pipeline != nil && (mr.Pipeline.Status != "success" || mr.Pipeline.SHA != commit) {
		return review, fmt.Errorf("MR pipeline is %s or references another commit; finalization is blocked", mr.Pipeline.Status)
	}
	review.Verified = len(approvals.Approved) > 0 && mr.Pipeline != nil
	return review, nil
}

func hotfixFetchReview(vcs, branch, commit string) (hotfixReview, error) {
	if vcs == "github" {
		output, err := hotfixRunVCS("gh", "pr", "view", branch, "--json", "url,state,isDraft,headRefOid,reviewDecision,statusCheckRollup")
		if err != nil {
			return hotfixReview{}, err
		}
		return hotfixGitHubReview(output, commit)
	}
	output, iid, err := hotfixGitLabMRDetails(branch, "", true)
	if err != nil {
		return hotfixReview{}, err
	}
	approvals, err := hotfixRunVCS("glab", "api", fmt.Sprintf("projects/:id/merge_requests/%d/approvals", iid))
	if err != nil {
		return hotfixReview{}, err
	}
	return hotfixGitLabReview(output, approvals, commit)
}

func hotfixVerifyReview(vcs, branch, commit string) error {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	tool := hotfixVCSTool(vcs)
	if tool == "" || app.CheckCommandExists(tool) != nil {
		ui.Note("Automatic review/CI verification unavailable. Configure vcs and install gh (GitHub) or glab (GitLab).")
		if !terminal.AskConfirmation("Have you manually verified review approval and successful CI for this exact commit?", false) {
			return fmt.Errorf("review/CI verification required; finalization postponed")
		}
		return nil
	}
	review, err := hotfixFetchReview(vcs, branch, commit)
	if review.URL != "" {
		ui.Note("Review: " + review.URL)
	}
	if err != nil {
		return err
	}
	if !review.Verified {
		ui.Note("No complete approval/CI evidence was returned by the provider.")
		if !terminal.AskConfirmation("Have you manually verified approval and successful CI for this exact commit?", false) {
			return fmt.Errorf("review/CI verification required; finalization postponed")
		}
	} else {
		ui.Success("Review approved and CI successful for the published commit")
	}
	return nil
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
	output, iid, err := hotfixGitLabMRDetails(branch, target, false)
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

func hotfixReviewLinks(branch, vcs, configuredTarget string) {
	ui := terminal.SymfonyStyle(terminal.Stdout, terminal.Stdin)
	target := hotfixReviewTarget(configuredTarget)
	if target != "" && terminal.AskConfirmation(fmt.Sprintf("Open or create the review to %s?", target), true) {
		if err := hotfixOpenReview(vcs, branch, target); err != nil {
			ui.Note("Branch remains published. Review could not be opened: " + err.Error())
		}
	}
	if terminal.AskConfirmation("Open CI for this hotfix?", false) {
		tool := hotfixVCSTool(vcs)
		var err error
		if tool == "" {
			err = fmt.Errorf("configure vcs to github or gitlab")
		} else if vcs == "github" {
			var output string
			output, err = hotfixRunVCS(tool, "run", "list", "--branch", branch, "--limit", "1", "--json", "url,databaseId")
			if err == nil {
				var runs []struct {
					URL string `json:"url"`
					ID  int    `json:"databaseId"`
				}
				err = json.Unmarshal([]byte(output), &runs)
				if err == nil && len(runs) > 0 {
					ui.Note("CI: " + runs[0].URL)
					_, err = hotfixRunVCS(tool, "run", "view", fmt.Sprint(runs[0].ID), "--web")
				} else if err == nil {
					ui.Note("No CI run found yet. Try again after the pipeline starts.")
				}
			}
		} else {
			var output string
			output, err = hotfixRunVCS(tool, "api", "projects/:id/pipelines?per_page=1&ref="+url.QueryEscape(branch))
			if err == nil {
				var pipelines []struct {
					URL string `json:"web_url"`
				}
				err = json.Unmarshal([]byte(output), &pipelines)
				if err == nil && len(pipelines) > 0 {
					ui.Note("CI: " + pipelines[0].URL)
					err = hotfixOpenURL(pipelines[0].URL)
				} else if err == nil {
					ui.Note("No CI pipeline found yet. Try again after the pipeline starts.")
				}
			}
		}
		if err != nil {
			ui.Note("Unable to open CI: " + err.Error())
		}
	}
}
