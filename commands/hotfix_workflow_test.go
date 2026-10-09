package commands

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
