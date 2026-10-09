package hotfix

import (
	"reflect"
	"testing"
)

func TestHotfixActions(t *testing.T) {
	for _, tt := range []struct {
		branch string
		prefix string
		want   []string
	}{
		{"main", "hotfix", []string{"start"}},
		{"hotfix/1.2.3", "hotfix", []string{"publish", "finish", "start"}},
		{"fix/1.2.3", "fix", []string{"publish", "finish", "start"}},
		{"hotfix-other", "hotfix", []string{"start"}},
	} {
		if got := hotfixActions(tt.branch, tt.prefix); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("hotfixActions(%q, %q) = %v, want %v", tt.branch, tt.prefix, got, tt.want)
		}
	}
}

func TestHotfixReleaseOptions(t *testing.T) {
	candidates := []string{"2.0.0-rc1", "1.2.3", "1.2.2"}
	options, choice := hotfixReleaseOptions(candidates, map[string]string{"1.2.3": "2026-10-09"})
	if choice != 1 || options[0] != "2.0.0-rc1 [prerelease]" || options[1] != "1.2.3 — 2026-10-09" {
		t.Fatalf("unexpected options/default: %v, %d", options, choice)
	}
	if options[len(options)-1] != "Other (I will provide it)" {
		t.Fatalf("manual choice must remain available for older tags: %v", options)
	}
	options, choice = hotfixReleaseOptions(nil, nil)
	if choice != 0 || len(options) != 1 {
		t.Fatalf("manual choice must remain available without tags: %v", options)
	}
}
