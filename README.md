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
