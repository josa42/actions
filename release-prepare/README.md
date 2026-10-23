# release-prepare

Prepare a release: pick the version, set it in your files, promote the
changelog, then commit, tag and push. Pair it with [release-publish][] to
create the GitHub release.

## Usage

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 1.2.3, or major, minor or patch
        required: true

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v5

      - id: prepare
        uses: josa42/actions/release-prepare@dist
        with:
          version: ${{ inputs.version }}
          version-files: |
            package.json: "version": "{version}"
            src/version.ts: export const VERSION = '{version}';

      - uses: josa42/actions/release-publish@dist
        with:
          tag: ${{ steps.prepare.outputs.tag }}
          notes: ${{ steps.prepare.outputs.notes }}
```

Start a release with `gh workflow run release -f version=minor`.

## Inputs

| Input             | Default                            | Description                                                                |
| ----------------- | ---------------------------------- | -------------------------------------------------------------------------- |
| `version`         |                                    | `1.2.3`, or `major`, `minor` or `patch`                                    |
| `version-files`   |                                    | Files to set the version in, one `path: template` per line                 |
| `npm-packages`    |                                    | Directories of npm packages to set the version in, one per line            |
| `only-if-changed` |                                    | Directories whose files only get the version if they changed, one per line |
| `changelog`       | `CHANGELOG.md`                     | Changelog to promote, skipped if the file does not exist                   |
| `commit-notes`    | `false`                            | Without a changelog, generate the notes from the commits                   |
| `tag-format`      | `v{version}`                       | Tag name                                                                   |
| `commit-message`  | `chore: bump version to {version}` | Message of the bump commit                                                 |
| `allow-branch`    | `false`                            | Allow releasing from a branch other than the default branch                |
| `github-token`    | `${{ github.token }}`              | Token for the GitHub API                                                   |

## Outputs

| Output    | Description                                                                       |
| --------- | --------------------------------------------------------------------------------- |
| `version` | The released version, e.g. `1.2.3`                                                |
| `tag`     | The release tag, e.g. `v1.2.3`                                                    |
| `sha`     | The tagged commit                                                                 |
| `notes`   | The promoted changelog section, or the notes from the commits with `commit-notes` |
| `changed` | The `only-if-changed` directories that got the version, one per line              |

## How it works

1. **Checks.** The workflow runs on the default branch, unless
   `allow-branch` is set. The tag does not exist yet. An explicit version is
   higher than the latest release.
2. **Version.** `major`, `minor` and `patch` bump the highest tag matching
   `tag-format`. Without one, they start from `0.0.0`.
3. **Files.** Every change is computed and checked before anything is
   written, so a failed check leaves the checkout untouched.
4. **Git.** The changes are committed as `github-actions[bot]` and tagged.
   Branch and tag are pushed atomically, both or neither. If no file changed,
   the current commit is tagged.

The checkout must be able to push, which `actions/checkout` does by default,
and the job needs `contents: write`.

### Version files

Each line is a path and a template, separated by the first `: `. The
template is the text around the version, with a `{version}` placeholder:

```yaml
version-files: |
  custom_components/x/manifest.json: "version": "{version}"
  custom_components/x/__init__.py: STRATEGY_VERSION = "{version}"
  card.js: const CARD_VERSION = "{version}";
```

A template matches from the start of a line, after indentation, and may be
followed by more text on that line. It must match exactly once, so a typo or
a moved line fails the release instead of shipping a stale version. The rest
of the file is left as is.

### npm packages

Each line of `npm-packages` is a directory with a `package.json`:

```yaml
npm-packages: |
  app
  extension
```

The top-level `"version"` of `package.json` is set, which must be the only
`"version"` in the file. If the directory has a `package-lock.json`, its
top-level `"version"` and the one of the root package, `packages[""]`, are
set too. The versions of dependencies and the formatting stay as they are.

### Only if changed

In a repository with several parts, `only-if-changed` keeps the version of a
part that did not change since the latest release:

```yaml
npm-packages: |
  app
  extension
only-if-changed: |
  extension
```

A version file or npm package inside one of these directories only gets the
new version if the directory differs from the latest release tag. Without a
previous release, every directory counts as changed. The checkout needs the
tags, so check out with `fetch-depth: 0`.

The `changed` output lists the directories that changed and got the
version, so later steps can publish only those.

### Changelog

The `## Unreleased` section becomes `## 1.2.3 - 2026-10-03` (UTC), and its
content becomes the `notes` output. `## [Unreleased]` becomes
`## [1.2.3] - 2026-10-03`.

The release fails if the changelog exists but has no `## Unreleased`
section, the section is empty, or there already is a section for the
version.

### Commit notes

Without a changelog, `commit-notes: true` generates the notes from the
subjects of the commits since the latest release, or of every commit for the
first release. Conventional Commits of the types `feat`, `fix` and `perf` are
grouped, breaking changes are marked:

```markdown
### Features

- **server:** add accounts
- **app:** drop the old import **(breaking)**

### Fixes

- handle empty imports
```

Other types, such as `chore`, `docs` or `refactor`, and commits that are no
Conventional Commits are left out. The notes are empty if nothing is left.
The checkout needs the history and tags, so check out with `fetch-depth: 0`.

<!-- internal links -->

[release-publish]: ../release-publish
