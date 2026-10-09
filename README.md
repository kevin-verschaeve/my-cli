# My CLI

A set of custom commands I use everyday to ease usage.

## Installation

From a clone of this repository, with Go and Make installed:

```sh
make install
```

This builds and installs `exo` into `$HOME/.local/bin`. Ensure that directory is
in your `PATH`. Re-run the same command to update the binary.

Customize the binary name, installation directory, and environment variable prefix:

```sh
PREFIX=MONCLI__ make install BINARY=mon_cli BINDIR="$HOME/.local/bin"
```

The prefix is embedded in the binary at compile time; it defaults to `MYCLI__`.
For example, this custom build reads `MONCLI__HOME` instead of `MYCLI__HOME`.
The default configuration directory remains `$HOME/mycli`, regardless of the
binary name or prefix. Installation does not create or overwrite configuration.

## Usage

- Show all available commands
```
exo
```

- Display help for a command
```
exo help <comman>
```

## Hotfix workflow

Run `exo hotfix` for an interactive menu, or choose a step explicitly:

1. `exo hotfix start`: choose the release and create a hotfix branch. The command exits so you can implement and commit the fix.
2. `exo hotfix publish`: push the current hotfix branch for review. Run it again to publish additional commits.
3. `exo hotfix finish`: after review, optionally publish the release tag and backport the fix.

Declining finalization postpones it; it does not delete your branch or commits.

Each step requires a clean working tree, no Git operation in progress, and an
accessible `origin` remote. Remote branches and tags are refreshed before changes.
Custom branch names must contain a valid release version; existing branches or
release tags are rejected at creation time.

The release picker shows the five latest version families, dates, and prerelease
labels. Stable releases are preferred over prereleases. You can enter an older
tag manually. Only `X.Y.Z` and `X.Y.Z-suffix` tags are supported.
The starting tag must be chosen interactively on every `exo hotfix start`;
it is never read from configuration. Verify which release is deployed before
confirming: the highest tag is not necessarily production.

Finalization requires the local hotfix tip to match the published branch. The
release confirmation displays its exact commit SHA. New release tags are
annotated and created on that explicit commit, regardless of the checked-out
branch. Existing local or remote tags must point to the same commit; tags are
never force-pushed or overwritten.

Publication pushes the hotfix branch and records its commit, without prompting
to open/create a review or open CI. Request the review separately. Set `vcs` to
`github` (requires authenticated `gh`) or `gitlab` (requires authenticated `glab`
for the hotfix workflow; older `open:pr`/`pipeline` commands still use `lab`).

Finalization checks the review's commit, draft/state, approvals, and CI status.
Known pending/failed checks or missing required approvals block release. If the
tool is unavailable or the provider has no complete approval/CI evidence, an
explicit manual verification is required. Provider/API errors do not silently
bypass verification. Review creation for backports remains available in review mode.

### Backports and recovery

Hotfix options are grouped under the `hotfix` object in configuration:

```json
"hotfix": {
    "prefix": "hotfix",
    "backport_targets": [],
    "backport_mode": "review"
}
```

`hotfix.prefix` defaults to `hotfix` and controls the hotfix branch prefix.
Move existing top-level `hotfix_prefix`, `hotfix_backport_targets`, and
`hotfix_backport_mode` settings into this object; the old keys are no longer read.

Choose backport targets with the multi-select prompt. `hotfix.backport_targets`
can list project-specific branches (for example `["develop", "main", "release/1.0"]`);
when empty, existing `develop` and `main`/`master` branches are suggested.
`hotfix.backport_mode` defaults to `review`:

- `review`: create a dedicated branch from each remote target, cherry-pick only
    the fix commits since the starting tag, push the branch, and open/create a PR/MR.
    Target branches are never pushed directly. Merge commits in the hotfix require
    a manual backport or explicitly choosing direct mode.
- `direct`: merge the validated SHA into a dedicated branch and push it to the
    target, without force. Use only when direct pushes are allowed by project policy.

Checkpoints are saved atomically in Git's common directory, not committed into
the repository. Once finalization starts, the validated SHA is frozen. Restart
with `exo hotfix resume` from any branch: successful tag publication and
completed backports are skipped. Failed pushes or review creation can be retried.
Review backports marked complete mean the PR/MR was opened, **not merged**.

On a conflict, resolve and stage files, then run `git cherry-pick --continue`
(review mode) or `git merge --continue` (direct mode), followed by
`exo hotfix resume`. Use the corresponding `--abort` to abandon the current Git
operation; resume will retry that backport. Do not delete or edit checkpointed
backport branches. A declined tag is recorded as skipped for that workflow.
Checkpoints are local to this clone; do not run concurrent hotfix commands.

### Status and recap

Each operation prints the recorded stage, starting release, published/validated
SHA, tag decision, per-target backport progress, last failure, and checked-out
branch. `exo hotfix status` reads these checkpoints without contacting the
remote or changing branches, so it works offline. If the current branch has no
checkpoint, it shows all tracked workflows. Status is a local progress report,
not a live deployment or PR-merge status.

Successful finalization restores the branch that was checked out when the command
started. Failures leave the backport branch checked out for inspection or conflict
resolution. No branch is deleted automatically, and no changes are auto-stashed.

## Developing

To build a new version of the CLI

```
go build -o $HOME/go/bin/exo
```

To embed a custom environment variable prefix without Make:

```sh
go build -ldflags "-X mycli/app.EnvPrefix=MONCLI__" -o ./mon_cli .
```

Setting `PREFIX=MONCLI__ go build` alone does not embed the value: Go does not
automatically read that environment variable. The Make target forwards `PREFIX`
to the linker using `-X`, which requires a string variable, not a constant.

To run a new version of a command before packaging it in the binary

```
go run main.go <command> <args>
```

## Configuration

Run `exo init` to create `$HOME/mycli/config.json` with default values, then
`exo config:edit` to customize it. Existing configuration is never overwritten.
The configuration file is created with owner-only read/write permissions.
Set `MYCLI__HOME` (or `<compiled prefix>HOME`) to use another configuration directory.

<details>
    <summary>Configuration Reference</summary>

    ```
    "preview_url_template": Url to open with the command `exo preview <pr-number>`. Place a `%s` placeholder to be replaced by the Pull Request number.
    "linear_organization": Project organization on [linear](https://linear.app).
    "linear_ticket_prefix": Prefix for your linear ticket. Defaults to environment variable `MYCLI__LINEAR_TICKET_PREFIX`.
    "daily_file": File to write your daily content.
    "pipeline_aliases": Open a pipeline using an alias. It is a map with `{"alias": "real pipeline name"}`.
    "pipeline_suffixes": If you need to add a suffix to the pipeline name.
    "pipeline_url_template": Url of the pipeline. Contains 3 placeholders in this order: "pipeline name", "pipeline environment", "pipeline suffix".
    ```
</details>
