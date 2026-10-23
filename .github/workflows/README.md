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
| [`shared-release.yml`](shared-release.yml) | Releases for apps, started by hand: version, tag and GitHub release |
| [`shared-esphome.yml`](shared-esphome.yml) | ESPHome device configs: yamllint and `esphome config` |
| [`shared-nvim-plugin.yml`](shared-nvim-plugin.yml) | Neovim plugins: stylua and plenary busted |
| [`shared-go.yml`](shared-go.yml) | Go modules: build, test and vet |
| [`shared-docker-publish.yml`](shared-docker-publish.yml) | Multi-arch Docker images pushed to Docker Hub |
| [`shared-extension-publish.yml`](shared-extension-publish.yml) | Browser extensions, unlisted in the Chrome Web Store and signed for Firefox |

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

Jobs: `build`, `test`, `vet` and `lint`, with the Go version from `go.mod`.
`lint` runs [`gofmt-lint`](../../gofmt-lint) and then the `lint` command, if
given.

| Input | Default | |
| --- | --- | --- |
| `working-directory` | `.` | Directory with the `go.mod` |
| `lint` | | Additional lint command, e.g. `make lint` |

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

With a `version`, the manifest is also tagged with the version and its
major.minor, `1.2.3` and `1.2`. Together with `ref`, that publishes a release
built by [`shared-release.yml`](#shared-releaseyml) from its tag:

```yaml
  docker:
    needs: release
    uses: josa42/actions/.github/workflows/shared-docker-publish.yml@main
    with:
      image: josa/myapp
      ref: ${{ needs.release.outputs.tag }}
      version: ${{ needs.release.outputs.version }}
      build-args: |
        VERSION=${{ needs.release.outputs.version }}
    secrets:
      DOCKERHUB_USERNAME: ${{ secrets.DOCKERHUB_USERNAME }}
      DOCKERHUB_TOKEN: ${{ secrets.DOCKERHUB_TOKEN }}
```

| Input | Default | |
| --- | --- | --- |
| `image` | | Docker Hub image name, required |
| `use-arm-runner` | `false` | Build arm64 on a native ARM runner |
| `ref` | the triggering ref | Ref to build, e.g. a release tag |
| `version` | | Also tag the image with the version and major.minor |
| `build-args` | | Build arguments, one `KEY=value` per line |

## shared-extension-publish.yml

```yaml
jobs:
  extension-publish:
    needs: extension
    if: startsWith(github.ref, 'refs/tags/v')
    uses: josa42/actions/.github/workflows/shared-extension-publish.yml@main
    with:
      name: myapp
      artifact: myapp-browser-extensions
      chrome-zip: myapp-chrome.zip
      firefox-zip: myapp-firefox.zip
      chrome-extension-id: ${{ vars.CHROME_EXTENSION_ID }}
    secrets:
      CHROME_CLIENT_ID: ${{ secrets.CHROME_CLIENT_ID }}
      CHROME_CLIENT_SECRET: ${{ secrets.CHROME_CLIENT_SECRET }}
      CHROME_REFRESH_TOKEN: ${{ secrets.CHROME_REFRESH_TOKEN }}
      CHROME_PUBLISHER_ID: ${{ secrets.CHROME_PUBLISHER_ID }}
      AMO_API_KEY: ${{ secrets.AMO_API_KEY }}
      AMO_API_SECRET: ${{ secrets.AMO_API_SECRET }}
      DIST_TOKEN: ${{ secrets.EXTENSION_DIST_TOKEN }}
```

Publishes the zips of an artifact from the same run, built by an earlier
job. Both stores get the extension unlisted: it is not found by search, only
by its link.

- **Chrome:** uploads to the existing item and submits it for review. The
  item has to be created once by hand in the developer dashboard, with the
  visibility "Unlisted".
- **Firefox:** signs at addons.mozilla.org on the unlisted channel and
  pushes `<name>/<name>-<version>.xpi`, `<name>/<name>.xpi` (the newest) and
  `<name>/updates.json` to the dist repository. The manifest must set
  `browser_specific_settings.gecko.update_url` to
  `<dist-url>/<name>/updates.json`, so that Firefox updates it.

The version comes from `manifest.json` in each zip. A version that is
recorded in the dist repository already (`chrome.json`, `updates.json`) is
skipped, so the job can run on every release tag; bump the version of the
extension to publish it.

A browser whose credentials are missing (the Chrome secrets and
`chrome-extension-id`, or the AMO secrets) is skipped with a notice, so one
store can be set up before the other.

| Input | Default | |
| --- | --- | --- |
| `name` | | Folder in the dist repository, required |
| `artifact` | | Artifact with the zips, required |
| `chrome-zip` / `firefox-zip` | | Zip in the artifact; empty skips that browser |
| `chrome-extension-id` | | Item in the Chrome Web Store |
| `dist-repository` | `josa42/extension-dist` | |
| `dist-url` | `https://josa.gesell.me/extension-dist` | Where GitHub Pages serves it |

`DIST_TOKEN` needs write access to the contents of the dist repository. The
Chrome secrets are an OAuth client of the Chrome Web Store API with its
refresh token and the publisher ID; the AMO secrets are the JWT issuer and
secret from addons.mozilla.org.

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

## shared-release.yml

```yaml
on:
  workflow_dispatch:
    inputs:
      version:
        description: 1.2.3, or major, minor or patch
        required: true
        default: patch

jobs:
  release:
    uses: josa42/actions/.github/workflows/shared-release.yml@main
    permissions:
      contents: write
      actions: read
    with:
      version: ${{ inputs.version }}
      require-workflow: ci.yml
      npm-packages: |
        app
        extension
      only-if-changed: |
        extension
```

Start a release with `gh workflow run release -f version=minor`. With
`require-workflow`, the release fails early unless a run of that workflow
succeeded for the commit being released, so a release never ships a commit
with failing checks. Unlike `shared-ha-release.yml`, it does not run the
checks again.

The `prepare` job uses [release-prepare] to set the version in the
`version-files` and `npm-packages`, date the `## Unreleased` section of
`CHANGELOG.md` if there is one, then commit, tag and push. Version files and
npm packages inside an `only-if-changed` directory keep their version unless
the directory changed since the latest release; the `changed` output lists
the ones that got it.

The `publish` job uses [release-publish] to create the GitHub release, with
no assets. The release notes are the changelog section, or without a
changelog the features, fixes and performance improvements among the
Conventional Commits since the latest release. If publishing fails, re-run
the failed job.

| Input | Default | |
| --- | --- | --- |
| `version` | | `1.2.3`, or `major`, `minor` or `patch` |
| `version-files` | | `path: template` lines, see [release-prepare] |
| `npm-packages` | | Directories of npm packages, see [release-prepare] |
| `only-if-changed` | | Directories that keep their version unless they changed |
| `commit-message` | `chore(release): {version}` | |
| `require-workflow` | | Workflow file, e.g. `ci.yml`, that must have succeeded |

| Output | |
| --- | --- |
| `version` | The released version, e.g. `1.2.3` |
| `tag` | The release tag, e.g. `v1.2.3` |
| `sha` | The tagged commit |
| `changed` | The `only-if-changed` directories that got the version, one per line |
| `url` | The release URL |

[release-prepare]: ../../release-prepare
[release-publish]: ../../release-publish
