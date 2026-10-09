package hotfix

import (
	"errors"
	"testing"
)

func TestHotfixGitHubReview(t *testing.T) {
	for _, tt := range []struct {
		name     string
		json     string
		blocked  bool
		verified bool
	}{
		{"approved", `{"url":"https://example/pr/1","state":"OPEN","headRefOid":"abc","reviewDecision":"APPROVED","statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}]}`, false, true},
		{"manual evidence needed", `{"state":"OPEN","headRefOid":"abc","statusCheckRollup":[]}`, false, false},
		{"draft", `{"state":"OPEN","headRefOid":"abc","isDraft":true}`, true, false},
		{"stale commit", `{"state":"OPEN","headRefOid":"old"}`, true, false},
		{"review pending", `{"state":"OPEN","headRefOid":"abc","reviewDecision":"REVIEW_REQUIRED"}`, true, false},
		{"CI pending", `{"state":"OPEN","headRefOid":"abc","statusCheckRollup":[{"status":"IN_PROGRESS"}]}`, true, false},
		{"CI failed", `{"state":"OPEN","headRefOid":"abc","statusCheckRollup":[{"state":"FAILURE"}]}`, true, false},
		{"malformed", `{`, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hotfixGitHubReview(tt.json, "abc")
			if (err != nil) != tt.blocked || got.Verified != tt.verified {
				t.Fatalf("review = %+v, %v", got, err)
			}
		})
	}
}

func TestHotfixGitLabReview(t *testing.T) {
	for _, tt := range []struct {
		name      string
		json      string
		approvals string
		blocked   bool
		verified  bool
	}{
		{"approved", `{"state":"opened","sha":"abc","head_pipeline":{"status":"success","sha":"abc"}}`, `{"approvals_left":0,"approved_by":[{}]}`, false, true},
		{"manual evidence needed", `{"state":"opened","sha":"abc"}`, `{}`, false, false},
		{"review pending", `{"state":"opened","sha":"abc"}`, `{"approvals_left":1}`, true, false},
		{"CI failed", `{"state":"opened","sha":"abc","head_pipeline":{"status":"failed","sha":"abc"}}`, `{}`, true, false},
		{"stale CI", `{"state":"opened","sha":"abc","head_pipeline":{"status":"success","sha":"old"}}`, `{}`, true, false},
		{"closed", `{"state":"closed","sha":"abc"}`, `{}`, true, false},
		{"malformed", `{`, `{}`, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hotfixGitLabReview(tt.json, tt.approvals, "abc")
			if (err != nil) != tt.blocked || got.Verified != tt.verified {
				t.Fatalf("review = %+v, %v", got, err)
			}
		})
	}
}

func TestHotfixSelectGitLabMR(t *testing.T) {
	output := `[{"iid":1,"target_branch":"main","state":"merged"},{"iid":2,"target_branch":"develop","state":"opened"},{"iid":3,"target_branch":"main","state":"opened"}]`
	if iid, err := hotfixSelectGitLabMR(output, "main", false); err != nil || iid != 3 {
		t.Fatalf("must select open MR for exact target: %d, %v", iid, err)
	}
	if _, err := hotfixSelectGitLabMR(output, "missing", false); !errors.Is(err, hotfixReviewNotFound) {
		t.Fatalf("missing target must not reuse another MR: %v", err)
	}
	if iid, err := hotfixSelectGitLabMR(`[{"iid":1,"state":"merged"}]`, "", true); err != nil || iid != 1 {
		t.Fatalf("finalization can inspect merged MR: %d, %v", iid, err)
	}
	if _, err := hotfixSelectGitLabMR(`{}`, "", false); err == nil || errors.Is(err, hotfixReviewNotFound) {
		t.Fatalf("API parse errors must not trigger MR creation: %v", err)
	}
}
