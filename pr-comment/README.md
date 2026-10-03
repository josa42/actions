# pr-comment

Post, update or delete a pull request comment.

## Usage

```yaml
- uses: josa42/actions/pr-comment@dist
  with:
    identifier: coverage
    body-path: coverage.md
```

The workflow needs `pull-requests: write` permission.

## Inputs

| Input          | Default               | Description                                         |
| -------------- | --------------------- | --------------------------------------------------- |
| `github-token` | `${{ github.token }}` | Token for the GitHub API                            |
| `body`         |                       | Comment body                                        |
| `body-path`    |                       | File containing the comment body                    |
| `action`       | `post`                | `post` or `delete`                                  |
| `identifier`   |                       | Identifies the comment across runs                  |
| `author`       |                       | Only update or delete comments by this user         |
| `pr`           |                       | Pull request number to use instead of the one found |
| `require-pr`   | `false`               | Fail if no pull request is found                    |

Posting needs `body` or `body-path`, not both. Deleting needs `identifier`.

Without a pull request the step is skipped with a notice, unless `require-pr`
is set. See [get-pr][] for how the pull request is found.

## Outputs

| Output        | Description                    |
| ------------- | ------------------------------ |
| `comment-id`  | ID of the posted comment       |
| `comment-url` | URL of the posted comment      |

## Identifier

Without an identifier every run adds a comment. With one, it is stored as a
hidden HTML comment in the body, and later runs:

- **post:** update the newest comment carrying it, and delete older ones left
  by parallel runs. An unchanged body is not rewritten.
- **delete:** delete every comment carrying it.

Comments by any user match. Set `author`, e.g. `github-actions[bot]`, to only
touch your own.

## Truncation

GitHub rejects comments longer than 65536 characters. Longer bodies are cut at
a line boundary, a code fence or `<details>` blocks left open by the cut are
closed, and a notice is appended. The identifier is added after truncating, so
it is never cut off.

<!-- internal links -->

[get-pr]: ../get-pr
