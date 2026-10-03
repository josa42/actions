# Changelog

## Unreleased

### Added

- **Go actions.** Actions can be written in Go against a small toolkit with no
  dependencies. Every push to `main` builds the binaries for Linux, macOS and
  Windows into the `dist` branch, so `@dist` runs without setting up Go. Any
  other ref builds from source.
- **get-pr.** Gets the pull request a workflow runs for, on pull request,
  comment, merge queue, push and deployment events.
- **get-changed-files.** Lists the files changed by a pull request or push,
  filtered by path patterns with the syntax of workflow `paths` filters.
- **pr-comment.** Posts, updates or deletes a pull request comment, identified
  across runs, and truncates bodies over GitHub's limit without breaking the
  markdown.
- **release-prepare.** Bumps the version in any file through templates,
  promotes the `## Unreleased` changelog section, then commits, tags and pushes
  the release atomically.
- **release-publish.** Creates or updates the GitHub release of a tag with its
  assets. Safe to re-run after a failed upload.
- **conventional-commits.** Checks that commit messages and pull request
  titles follow Conventional Commits, with configurable types, scopes and
  style rules. Problems show as annotations, in the job summary and in a pull
  request comment that disappears once they are fixed.
- **Shared workflows for Home Assistant, ESPHome and Neovim.** The workflows
  of josa42/gha-workflows moved here, prefixed with `shared-`:
  `shared-ha-integration`, `shared-ha-plugin`, `shared-ha-blueprints`,
  `shared-ha-release`, `shared-esphome` and `shared-nvim-plugin`. They run on
  Node.js 24 action versions. See the [shared workflows](.github/workflows).
- **shared-ha-release builds bundled cards.** The `build` input runs a build
  command on the tagged commit before the release assets are attached.
- **esphome-lint.** Lints ESPHome device configs with yamllint and
  `esphome config`, with errors as annotations on the offending line.
- **gofmt-lint.** Checks that Go files are formatted with gofmt, with
  annotations on the lines gofmt would change. Files are never written.
- **shared-go lints.** A new `lint` job runs `gofmt-lint` and, if given, the
  command from the `lint` input.

### Changed

- **Shared workflows are prefixed with `shared-`.** `go.yml` is now
  `shared-go.yml` and `docker-publish.yml` is now `shared-docker-publish.yml`,
  so they stand apart from this repository's own workflows. Callers need to
  update the path in `uses:`.
- **Actions run on Node.js 24.** Every action this repository uses was moved to
  a version that runs on Node.js 24, since Node.js 20 is deprecated on GitHub
  runners.
