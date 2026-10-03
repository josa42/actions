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
- **esphome-lint.** Lints ESPHome device configs with yamllint and
  `esphome config`, with errors as annotations on the offending line.

### Changed

- **Actions run on Node.js 24.** Every action this repository uses was moved to
  a version that runs on Node.js 24, since Node.js 20 is deprecated on GitHub
  runners.
