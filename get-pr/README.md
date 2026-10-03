# get-pr

Get the pull request the workflow runs for.

## Usage

```yaml
- id: pr
  uses: josa42/actions/get-pr@dist

- if: steps.pr.outputs.found == 'true'
  run: echo "PR #${{ steps.pr.outputs.number }} targets ${{ steps.pr.outputs.base-ref }}"
```

## Inputs

| Input          | Default               | Description                                                  |
| -------------- | --------------------- | ------------------------------------------------------------ |
| `github-token` | `${{ github.token }}` | Token for the GitHub API                                     |
| `pr`           |                       | Pull request number to use instead of the one found          |
| `require-pr`   | `false`               | Fail if no pull request is found                             |

## Outputs

| Output     | Description                                     |
| ---------- | ----------------------------------------------- |
| `found`    | `true` or `false`                               |
| `number`   | Pull request number                             |
| `url`      | Pull request URL                                |
| `title`    | Title                                           |
| `head-ref` | Head branch                                     |
| `head-sha` | Head commit SHA                                 |
| `base-ref` | Base branch                                     |
| `base-sha` | Base commit SHA                                 |
| `draft`    | `true` or `false`                               |
| `labels`   | Label names, one per line                       |
| `json`     | The pull request as returned by the GitHub API  |

Without a pull request only `found` is set.

## Finding the pull request

| Event                                     | Pull request                                    |
| ----------------------------------------- | ----------------------------------------------- |
| `pull_request`, `pull_request_target`     | The event's pull request                        |
| `issue_comment`                           | The commented pull request, none for issues     |
| `merge_group`                             | The pull request from the merge queue branch    |
| `push`, `deployment`, `deployment_status` | The open pull request containing the commit     |
| Anything else                             | None                                            |

The pull request is always fetched from the API, so labels, draft state and
head are current, not a snapshot from when the event fired.
