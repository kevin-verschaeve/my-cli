package hotfix

import (
	"errors"
	"testing"
)

func TestHotfixSelectGitLabMR(t *testing.T) {
	output := `[{"iid":1,"target_branch":"main","state":"merged"},{"iid":2,"target_branch":"develop","state":"opened"},{"iid":3,"target_branch":"main","state":"opened"}]`
	if iid, err := hotfixSelectGitLabMR(output, "main"); err != nil || iid != 3 {
		t.Fatalf("must select open MR for exact target: %d, %v", iid, err)
	}
	if _, err := hotfixSelectGitLabMR(output, "missing"); !errors.Is(err, hotfixReviewNotFound) {
		t.Fatalf("missing target must not reuse another MR: %v", err)
	}
	if _, err := hotfixSelectGitLabMR(`[{"iid":1,"state":"merged"}]`, ""); !errors.Is(err, hotfixReviewNotFound) {
		t.Fatalf("backports must not reuse merged MR: %v", err)
	}
	if _, err := hotfixSelectGitLabMR(`{}`, ""); err == nil || errors.Is(err, hotfixReviewNotFound) {
		t.Fatalf("API parse errors must not trigger MR creation: %v", err)
	}
}
