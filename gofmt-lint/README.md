# gofmt-lint

Check that Go files are formatted with `gofmt`. Every block gofmt would change
gets an error annotation on its file and lines, with the expected code in the
message. Syntax errors are annotated with their line and column.

Files are only read, never written, so later steps see the checkout as it was.

## Usage

```yaml
jobs:
  gofmt:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: josa42/actions/gofmt-lint@dist
```

`@dist` runs a prebuilt binary. Any other ref builds from source and needs
`actions/setup-go` first.

## Inputs

| Input               | Default | Description                      |
| ------------------- | ------- | -------------------------------- |
| `working-directory` | `.`     | Directory to check, recursively  |

## Outputs

| Output  | Description                                                       |
| ------- | ----------------------------------------------------------------- |
| `files` | Unformatted files, one per line, relative to the repository root  |

## How it works

All `.go` files under `working-directory` are checked, except in `vendor`,
`testdata` and directories starting with `.` or `_`, the same ones the go tool
ignores.

Each file is formatted in memory with [`go/format`][go-format], the package
`gofmt` is built on, and compared line by line with the original. The job
summary lists the unformatted files.

## Limitations

- **Toolkit Go version.** Formatting uses the Go version the action was built
  with, not the one in your `go.mod`. gofmt output rarely changes between
  versions.
- **No `-s`.** The simplifications of `gofmt -s` are not checked.

<!-- external links -->

[go-format]: https://pkg.go.dev/go/format
