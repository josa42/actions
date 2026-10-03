# conventional-commits

Check that commit messages and pull request titles follow
[Conventional Commits][spec].

## Usage

```yaml
on:
  push:
    branches: [main]
  pull_request:

jobs:
  commits:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: josa42/actions/conventional-commits@dist
```

No checkout is needed, the commits come from the GitHub API.
`pull-requests: write` is only needed for the comment.

## Inputs

| Input                | Default               | Description                                                     |
| -------------------- | --------------------- | --------------------------------------------------------------- |
| `types`              | see below             | Allowed types, one per line                                     |
| `scopes`             |                       | Allowed scopes, one per line. Any scope when empty.             |
| `max-length`         | `72`                  | Maximum length of the header, `0` to disable                    |
| `lowercase`          | `true`                | The description starts with a lowercase letter                  |
| `no-trailing-period` | `true`                | The description does not end with a period                      |
| `comment`            | `true`                | List problems in a pull request comment                         |
| `github-token`       | `${{ github.token }}` | Token for the GitHub API                                        |

The default types are `feat`, `fix`, `docs`, `refactor`, `test`, `chore`,
`build`, `ci`, `perf` and `style`. Setting `types` replaces them.

Scopes are always optional. With `scopes` set, a scope must be one of them,
which catches typos like `feat(gihub):`.

## What is checked

| Event                                 | Messages                                              |
| ------------------------------------- | ----------------------------------------------------- |
| `pull_request`, `pull_request_target` | The pull request title and every commit, at most 250  |
| `push`                                | The pushed commits, from the default branch for a new branch |
| Anything else                         | Nothing                                               |

Both are checked on pull requests because either can land on the base
branch: a squash merge uses the title, a merge or rebase keeps the commits.

## Rules

A message is `type(scope)!: description`, optionally followed by a blank
line, a body and footers. On top of the specification:

- the description starts lowercase and does not end with a period
- the header is at most `max-length` characters
- `BREAKING CHANGE:` and `BREAKING-CHANGE:` footers are uppercase

Some messages get special treatment:

- **Merge commits** are skipped. Their message is written by git, and the
  commits they bring in are checked themselves.
- **Reverts** with git's default message, `Revert "<header>"`, are valid if
  the quoted header is.
- **`fixup!`, `squash!` and `amend!` commits** always fail. Squash them with
  `git rebase -i --autosquash` before merging.

## Reporting

Every invalid message gets an error annotation listing all its problems, and
the job summary shows every message checked.

On pull requests, the problems are also posted as a comment, updated on every
run and deleted once all messages are valid. If the token cannot comment, for
example on pull requests from forks, a warning is logged and the check result
stands.

<!-- external links -->

[spec]: https://www.conventionalcommits.org/en/v1.0.0/
