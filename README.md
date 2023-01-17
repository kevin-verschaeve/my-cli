# My CLI

A set of custom commands I use everyday to ease usage.

## Usage

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
    
    - `preview_url_template`: Url to open with the command `mycli preview <pr-number>`. Place a `%s` placeholder to be replaced by the Pull Request number.
    - `linear_organization`: Project organization on [linear](https://linear.app).
    - `linear_ticket_prefix`: Prefix for your linear ticket. Defaults to environment variable `MYCLI__LINEAR_TICKET_PREFIX`.
    - `daily_directory`: Directory to store the daily file (default to your home if null).
    - `daily_file`: File to write your daily content.
    - `pipeline_aliases`: Open a pipeline using an alias. It is a map with `{"alias": "real pipeline name"}`.
    - `pipeline_suffixes`: If you need to add a suffix to the pipeline name.
    - `pipeline_url_template`: Url of the pipeline. Contains 3 placeholders in this order: "pipeline name", "pipeline environment", "pipeline suffix". 
</details>
