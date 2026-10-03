# get-changed-files

Get the files changed by the pull request or push, optionally filtered by
path patterns. No checkout or fetch depth is needed, the list comes from the
GitHub API.

## Usage

```yaml
- id: changed
  uses: josa42/actions/get-changed-files@dist
  with:
    paths: |
      src
      !src/**/*.test.ts

- if: steps.changed.outputs.matched == 'true'
  run: npm run lint -- $(jq -r '.[]' <<< '${{ steps.changed.outputs.files }}')
```

## Inputs

| Input          | Default               | Description                                         |
| -------------- | --------------------- | --------------------------------------------------- |
| `paths`        |                       | Path patterns, one per line. Unset means all files. |
| `github-token` | `${{ github.token }}` | Token for the GitHub API                            |
| `pr`           |                       | Pull request number to use instead of the one found |
| `require-pr`   | `false`               | Fail if no pull request is found                    |

## Outputs

| Output    | Description                                                       |
| --------- | ----------------------------------------------------------------- |
| `files`   | JSON array of the matching files that exist after the change      |
| `matched` | `true` if any change matches, including deleted and renamed files |
| `count`   | Number of entries in `files`                                      |

Deleted files are not in `files`, since tools run on the list would fail on
them. They still count for `matched`, as does the old path of a renamed file,
so deleting or moving a file out of `src` matches `src`.

## Which changes

| Run                                     | Files                                           |
| --------------------------------------- | ----------------------------------------------- |
| With a pull request (see [get-pr][])    | All files of the pull request, at most 3000     |
| `push` without a pull request           | Changes from `before` to `after`, at most 300   |
| `push` creating a branch                | Changes from the default branch to `after`      |
| Anything else                           | None                                            |

A warning is logged when the API limit is reached.

## Path patterns

The syntax of [`on.push.paths`][paths]:

| Pattern  | Matches                                       |
| -------- | --------------------------------------------- |
| `*`      | Any characters except `/`                     |
| `**`     | Any characters including `/`                  |
| `?`      | One character except `/`                      |
| `[a-z]`  | One character of a class, `[!a-z]` negated    |
| `\*`     | A literal `*`                                 |
| `!`      | At the start, excludes matching paths         |

Later patterns override earlier ones. If the first pattern is an exclusion,
everything else is included. Dot files match like any other file.

A pattern without wildcards, or with a trailing `/`, also matches everything
below it: `src` matches `src/app/main.ts`.

<!-- internal links -->

[get-pr]: ../get-pr

<!-- external links -->

[paths]: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#filter-pattern-cheat-sheet
