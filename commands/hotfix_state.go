package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mycli/app"
)

type hotfixBackportState struct {
	Target       string `json:"target"`
	Mode         string `json:"mode"`
	Branch       string `json:"branch"`
	Phase        string `json:"phase"`
	ResultCommit string `json:"result_commit,omitempty"`
}

type hotfixState struct {
	Branch           string                `json:"branch"`
	Base             string                `json:"base,omitempty"`
	BaseCommit       string                `json:"base_commit,omitempty"`
	Commit           string                `json:"commit,omitempty"`
	Phase            string                `json:"phase"`
	Tag              string                `json:"tag,omitempty"`
	TagStatus        string                `json:"tag_status,omitempty"`
	Backports        []hotfixBackportState `json:"backports,omitempty"`
	BackportsPlanned bool                  `json:"backports_planned"`
}

type hotfixStore struct {
	path      string
	Workflows map[string]*hotfixState
}

func hotfixLoadStore() (*hotfixStore, error) {
	dir, err := app.RunGitCommand("rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	store := &hotfixStore{
		path:      filepath.Join(strings.TrimSpace(dir), "mycli-hotfix.json"),
		Workflows: make(map[string]*hotfixState),
	}
	data, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &store.Workflows); err != nil {
		return nil, fmt.Errorf("cannot read hotfix checkpoint %s: %w", store.path, err)
	}
	if store.Workflows == nil {
		store.Workflows = make(map[string]*hotfixState)
	}
	for branch, state := range store.Workflows {
		if state == nil || state.Branch != branch {
			return nil, fmt.Errorf("invalid checkpoint for hotfix %q", branch)
		}
	}
	return store, nil
}

func (store *hotfixStore) save() error {
	data, err := json.MarshalIndent(store.Workflows, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(store.path), ".mycli-hotfix-*")
	if err != nil {
		return fmt.Errorf("cannot save hotfix checkpoint: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), store.path)
}

func (store *hotfixStore) state(branch string) *hotfixState {
	state := store.Workflows[branch]
	if state == nil {
		state = &hotfixState{Branch: branch, Phase: "started"}
		store.Workflows[branch] = state
	}
	return state
}

func hotfixDefaultAction(state *hotfixState) string {
	if state.Phase == "published" || state.Phase == "finishing" {
		return "finish"
	}
	return "publish"
}

func hotfixFindBase(state *hotfixState) error {
	if state.BaseCommit != "" {
		if _, err := app.RunGitCommand("merge-base", "--is-ancestor", state.BaseCommit, "refs/heads/"+state.Branch); err != nil {
			return fmt.Errorf("hotfix branch no longer descends from its recorded starting release: %w", err)
		}
		return nil
	}
	// Support branches created before checkpoints existed. Exclude the branch tip
	// so a release tag already placed on it cannot become its own starting point.
	base, err := app.RunGitCommand("describe", "--tags", "--abbrev=0", "refs/heads/"+state.Branch+"^")
	if err != nil {
		return fmt.Errorf("cannot infer the starting release for %s: %w", state.Branch, err)
	}
	state.Base = strings.TrimSpace(base)
	commit, err := app.RunGitCommand("rev-parse", "--verify", "refs/tags/"+state.Base+"^{commit}")
	if err != nil {
		return err
	}
	state.BaseCommit = strings.TrimSpace(commit)
	return hotfixFindBase(state)
}
