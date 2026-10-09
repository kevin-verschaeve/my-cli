package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfigHotfix(t *testing.T) {
	for _, tt := range []struct {
		name string
		json string
		want HotfixConfig
	}{
		{
			name: "nested options",
			json: `{"hotfix":{"prefix":"fix","backport_targets":["develop","main"],"backport_mode":"review"}}`,
			want: HotfixConfig{Prefix: "fix", BackportTargets: []string{"develop", "main"}, BackportMode: "review"},
		},
		{name: "omitted options", json: `{}`},
		{name: "empty object", json: `{"hotfix":{}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("MYCLI__HOME", home)
			if err := os.WriteFile(filepath.Join(home, CONFIG_FILE), []byte(tt.json), 0600); err != nil {
				t.Fatal(err)
			}
			config, err := LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(config.Hotfix, tt.want) {
				t.Fatalf("Hotfix = %+v, want %+v", config.Hotfix, tt.want)
			}
		})
	}
}
