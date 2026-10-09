# My CLI

A set of custom commands I use everyday to ease usage.

## Usage

- Show all available commands
```
mycli
```

- Display help for a command
```
mycli help <comman>
```

## Hotfix workflow

Run `mycli hotfix` for an interactive menu, or choose a step explicitly:

1. `mycli hotfix start`: choose the release and create a hotfix branch. The command exits so you can implement and commit the fix.
2. `mycli hotfix publish`: push the current hotfix branch for review. Run it again to publish additional commits.
3. `mycli hotfix finish`: after review, optionally publish the release tag and backport the fix.

Declining finalization postpones it; it does not delete your branch or commits.

Each step requires a clean working tree, no Git operation in progress, and an
accessible `origin` remote. Remote branches and tags are refreshed before changes.
Custom branch names must contain a valid release version; existing branches or
release tags are rejected at creation time.

The release picker shows the five latest version families, dates, and prerelease
labels. Stable releases are preferred over prereleases. You can enter an older
tag manually. Only `X.Y.Z` and `X.Y.Z-suffix` tags are supported.
Set `hotfix_production_tag` to the known deployed tag to suggest it by default,
including when it is older than the recent releases. This is an explicit hint,
not an automatic deployment lookup: keep it up to date.

Finalization requires the local hotfix tip to match the published branch. The
release confirmation displays its exact commit SHA. New release tags are
annotated and created on that explicit commit, regardless of the checked-out
branch. Existing local or remote tags must point to the same commit; tags are
never force-pushed or overwritten.

After publication you can open or create a review and open CI. Set `vcs` to
`github` (requires authenticated `gh`) or `gitlab` (requires authenticated `glab`
for the hotfix workflow; older `open:pr`/`pipeline` commands still use `lab`).
`hotfix_review_target` chooses the review target; otherwise the first existing
branch among `main`, `master`, and `develop` is used.

Finalization checks the review's commit, draft/state, approvals, and CI status.
Known pending/failed checks or missing required approvals block release. If the
tool is unavailable or the provider has no complete approval/CI evidence, an
explicit manual verification is required. Provider/API errors do not silently
bypass verification. Review and CI actions are optional after a successful push.

### Backports and recovery

Choose backport targets with the multi-select prompt. `hotfix_backport_targets`
can list project-specific branches (for example `["develop", "main", "release/1.0"]`);
when empty, existing `develop` and `main`/`master` branches are suggested.
`hotfix_backport_mode` defaults to `review`:

- `review`: create a dedicated branch from each remote target, cherry-pick only
    the fix commits since the starting tag, push the branch, and open/create a PR/MR.
    Target branches are never pushed directly. Merge commits in the hotfix require
    a manual backport or explicitly choosing direct mode.
- `direct`: merge the validated SHA into a dedicated branch and push it to the
    target, without force. Use only when direct pushes are allowed by project policy.

Checkpoints are saved atomically in Git's common directory, not committed into
the repository. Once finalization starts, the validated SHA is frozen. Restart
with `mycli hotfix resume` from any branch: successful tag publication and
completed backports are skipped. Failed pushes or review creation can be retried.
Review backports marked complete mean the PR/MR was opened, **not merged**.

On a conflict, resolve and stage files, then run `git cherry-pick --continue`
(review mode) or `git merge --continue` (direct mode), followed by
`mycli hotfix resume`. Use the corresponding `--abort` to abandon the current Git
operation; resume will retry that backport. Do not delete or edit checkpointed
backport branches. A declined tag is recorded as skipped for that workflow.
Checkpoints are local to this clone; do not run concurrent hotfix commands.

## Developing

To build a new version of the CLI

```
go build -o $HOME/go/bin/mycli
```

To run a new version of a command before packaging it in the binary

```
go run main.go <command> <args>
```

## Configuration

Copy the `config.json.dist` file to `config.json` and fill it with correct data.

<details>
    <summary>Configuration Reference</summary>
    
    ```
    "preview_url_template": Url to open with the command `mycli preview <pr-number>`. Place a `%s` placeholder to be replaced by the Pull Request number.
    "linear_organization": Project organization on [linear](https://linear.app).
    "linear_ticket_prefix": Prefix for your linear ticket. Defaults to environment variable `MYCLI__LINEAR_TICKET_PREFIX`.
    "daily_file": File to write your daily content.
    "pipeline_aliases": Open a pipeline using an alias. It is a map with `{"alias": "real pipeline name"}`.
    "pipeline_suffixes": If you need to add a suffix to the pipeline name.
    "pipeline_url_template": Url of the pipeline. Contains 3 placeholders in this order: "pipeline name", "pipeline environment", "pipeline suffix". 
    ```
</details>
