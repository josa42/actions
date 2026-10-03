# Shared workflows

Reusable workflows shared across my repositories. Workflows without the
`shared-` prefix are this repository's own.

Call one with `uses: josa42/actions/.github/workflows/<file>@main`.

| Workflow | Use for |
| --- | --- |
| [`shared-ha-integration.yml`](shared-ha-integration.yml) | Home Assistant custom integrations: ruff, pytest, HACS and hassfest |
| [`shared-ha-plugin.yml`](shared-ha-plugin.yml) | Home Assistant frontend cards: syntax check and HACS |
| [`shared-ha-blueprints.yml`](shared-ha-blueprints.yml) | Home Assistant blueprint repositories driven by a Makefile |
| [`shared-ha-release.yml`](shared-ha-release.yml) | Releases for integrations and cards, started by hand |
| [`shared-esphome.yml`](shared-esphome.yml) | ESPHome device configs: yamllint and `esphome config` |
| [`shared-nvim-plugin.yml`](shared-nvim-plugin.yml) | Neovim plugins: stylua and plenary busted |
| [`shared-go.yml`](shared-go.yml) | Go modules: build, test and vet |
| [`shared-docker-publish.yml`](shared-docker-publish.yml) | Multi-arch Docker images pushed to Docker Hub |

Each workflow detects what the calling repository actually contains and skips
the jobs that do not apply, so most callers need no inputs at all.

## shared-ha-integration.yml

```yaml
jobs:
  ci:
    uses: josa42/actions/.github/workflows/shared-ha-integration.yml@main
```

Jobs: `lint` (ruff over `custom_components` and the tests directory), `test`
(pytest) and `validate` (HACS plus hassfest).

`lint` is skipped without a ruff config, `test` without a tests directory, and
`validate` without a `hacs.json`. The test requirements file is picked from
`requirements_test.txt`, `requirements-test.txt`, `requirements_dev.txt` or
`requirements-dev.txt`, whichever exists first.

| Input | Default | |
| --- | --- | --- |
| `python_version` | `"3.14"` | |
| `tests` | `tests` | Directory passed to pytest |
| `pytest_args` | `-v --tb=short` | |
| `requirements` | detected | Test requirements file |
| `hacs_category` | `integration` | |
| `hacs_ignore` | | Space separated HACS checks to ignore |
| `skip_lint` / `skip_test` / `skip_validate` | detected | Set to `"true"` to force off |

## shared-ha-plugin.yml

```yaml
jobs:
  ci:
    uses: josa42/actions/.github/workflows/shared-ha-plugin.yml@main
```

Runs `npm run check` when `package.json` defines that script, otherwise
`node --check` on the entry point named by `hacs.json`.

| Input | Default | |
| --- | --- | --- |
| `node_version` | `"22"` | |
| `file` | `hacs.json` `filename` | Card entry point |
| `hacs_ignore` | | Space separated HACS checks to ignore |
| `skip_lint` / `skip_validate` | | Set to `"true"` to force off |

## shared-ha-blueprints.yml

```yaml
jobs:
  ci:
    uses: josa42/actions/.github/workflows/shared-ha-blueprints.yml@main
```

Runs `make venv` and then each target in `targets`. Every target runs even
after a failure, so one broken blueprint does not hide an out of date README.
Targets the Makefile does not define are skipped.

| Input | Default | |
| --- | --- | --- |
| `python_version` | `"3.12"` | |
| `requirements` | `requirements-dev.txt` | Used for the pip cache key |
| `targets` | `lint validate readme-check` | |

## shared-esphome.yml

```yaml
jobs:
  ci:
    uses: josa42/actions/.github/workflows/shared-esphome.yml@main
```

Runs [`esphome-lint`](../../esphome-lint):
yamllint over the config directory, then `esphome config` for every file in
`files` with a top-level `esphome:` key. Missing secrets are filled with dummy
values, so no `secrets.yaml` needs to be committed.

| Input | Default | |
| --- | --- | --- |
| `working_directory` | `.` | Root of the ESPHome config |
| `files` | `*.yaml` | Newline separated globs of device files |
| `esphome_version` | `latest` | |
| `secrets_file` | | Secrets file used instead of dummy secrets |
| `secrets` | | YAML mapping merged on top of the dummy secrets |

## shared-nvim-plugin.yml

```yaml
jobs:
  test:
    uses: josa42/actions/.github/workflows/shared-nvim-plugin.yml@main
```

Jobs: `lint` (stylua) and `test` (plenary busted). `lint` is skipped without a
`stylua.toml`.

| Input | Default | |
| --- | --- | --- |
| `specs` | `specs` | Directory with the specs |
| `skip_lint` / `skip_test` | detected | Set to `"true"` to force off |

## shared-go.yml

```yaml
jobs:
  go:
    uses: josa42/actions/.github/workflows/shared-go.yml@main
```

Jobs: `build`, `test` and `vet`, with the Go version from `go.mod`.

| Input | Default | |
| --- | --- | --- |
| `working-directory` | `.` | Directory with the `go.mod` |

## shared-docker-publish.yml

```yaml
jobs:
  docker:
    uses: josa42/actions/.github/workflows/shared-docker-publish.yml@main
    with:
      image: josa/myapp
    secrets:
      DOCKERHUB_USERNAME: ${{ secrets.DOCKERHUB_USERNAME }}
      DOCKERHUB_TOKEN: ${{ secrets.DOCKERHUB_TOKEN }}
```

Builds the image for amd64 and arm64 and pushes both, plus a multi-arch
manifest tagged `latest`.

| Input | Default | |
| --- | --- | --- |
| `image` | | Docker Hub image name, required |

## shared-ha-release.yml

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 1.2.3, or major, minor or patch
        required: true
        default: patch

jobs:
  ci:
    uses: ./.github/workflows/ci.yml
    permissions:
      contents: read

  release:
    needs: ci
    uses: josa42/actions/.github/workflows/shared-ha-release.yml@main
    permissions:
      contents: write
    with:
      version: ${{ inputs.version }}
```

Start a release with `gh workflow run release -f version=minor`. The caller's
`ci.yml` needs a `workflow_call:` trigger, so the release runs the same checks
as every push.

The `prepare` job uses [release-prepare] to bump the version, date the
`## Unreleased` section of `CHANGELOG.md` if there is one, then commit, tag and
push. For an integration, `custom_components/<domain>/manifest.json` is always
bumped. Other files holding the version go in `version_files`:

```yaml
    with:
      version: ${{ inputs.version }}
      version_files: |
        custom_components/x/__init__.py: STRATEGY_VERSION = "{version}"
```

The `publish` job checks out the tag and uses [release-publish]. For an
integration it zips `custom_components/<domain>`, for anything else it attaches
the file named by `hacs.json`. A card that is bundled before it ships passes
its build command and the built file:

```yaml
    with:
      version: ${{ inputs.version }}
      build: npm ci && npm run build
      files: dist/card.js
```

The build runs on the tagged commit, after the version bump. The release notes are the changelog section, or
generated from the commits without a changelog. If publishing fails, re-run the
failed job.

| Input | Default | |
| --- | --- | --- |
| `version` | | `1.2.3`, or `major`, `minor` or `patch` |
| `version_files` | | Extra `path: template` lines, see [release-prepare] |
| `files` | detected | Release assets |
| `build` | | Command that builds the release assets, Node is set up for it |
| `node_version` | `"22"` | Node version for `build` |
| `draft` | `false` | |
| `prerelease` | `false` | |

[release-prepare]: ../../release-prepare
[release-publish]: ../../release-publish
