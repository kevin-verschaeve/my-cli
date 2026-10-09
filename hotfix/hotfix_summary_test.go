package hotfix

import (
	"strings"
	"testing"
)

func TestHotfixSummary(t *testing.T) {
	state := &hotfixState{
		Branch: "hotfix/1.0.1", Base: "1.0.0", Commit: "abc", Phase: "finishing",
		Tag: "1.0.1", TagStatus: "published", BackportsPlanned: true, LastError: "push rejected",
		Backports: []hotfixBackportState{
			{Target: "main", Mode: "review", Branch: "backport/main", Phase: "done"},
			{Target: "develop", Mode: "direct", Branch: "backport/develop", Phase: "applied"},
			{Target: "release/1.0", Mode: "review", Phase: "done", Outcome: "already-present"},
		},
	}
	got := hotfixSummary(state, "backport/develop")
	for _, want := range []string{
		"Hotfix: hotfix/1.0.1", "Starting release: 1.0.0", "commit: abc", "1.0.1 (published)",
		"approval/merge still required", "develop [direct]: applied", "fix already present",
		"Last failure: push rejected", "Checked-out branch: backport/develop", "exo hotfix resume",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q:\n%s", want, got)
		}
	}
	state.Phase, state.Backports = "completed", nil
	state.TagStatus = "skipped"
	got = hotfixSummary(state, state.Branch)
	if !strings.Contains(got, "1.0.1 (skipped)") || !strings.Contains(got, "none selected") || strings.Contains(got, "exo hotfix resume") {
		t.Fatalf("misleading completed summary:\n%s", got)
	}
}

func TestHotfixSummaryInfersReleaseTagFromBranch(t *testing.T) {
	for _, tt := range []struct {
		branch string
		tag    string
		want   string
	}{
		{"hotfix/1.0.1", "", "1.0.1"},
		{"fix/1.0.1-rc1", "", "1.0.1-rc1"},
		{"release/hotfix/1.0.1", "", "1.0.1"},
		{"hotfix/1.0.1", "1.0.2", "1.0.2"},
	} {
		t.Run(tt.branch+tt.tag, func(t *testing.T) {
			state := &hotfixState{Branch: tt.branch, Tag: tt.tag, Phase: "started"}
			got := hotfixSummary(state, tt.branch)
			if !strings.Contains(got, "Release tag: "+tt.want+" (not decided)") {
				t.Fatalf("expected release tag %q in summary:\n%s", tt.want, got)
			}
			if state.Tag != tt.tag || state.TagStatus != "" {
				t.Fatal("displaying status must not mutate the checkpoint or publication decision")
			}
		})
	}
}
