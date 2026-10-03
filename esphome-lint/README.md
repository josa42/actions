# esphome-lint

Lint [ESPHome][esphome] device configs in CI. Runs [yamllint][yamllint] for
style and `esphome config` for full ESPHome validation, and reports errors as
annotations on the offending file and line.

No toolchain is installed and nothing is compiled. To build firmware, use
[esphome/build-action][build-action].

## Usage

`.github/workflows/esphome.yml`

```yaml
name: ESPHome

on:
  push:
    branches: [main]
  pull_request:

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: josa42/actions/esphome-lint@main
        with:
          working-directory: esphome
```

The action checks out the repository itself, so no `actions/checkout` step is
needed.

## Inputs

| Input               | Default   | Description                                                                 |
| ------------------- | --------- | --------------------------------------------------------------------------- |
| `working-directory` | `.`       | Root of the ESPHome config, relative to the repository root                 |
| `files`             | `*.yaml`  | Newline separated globs of device files, relative to `working-directory`    |
| `esphome-version`   | `latest`  | ESPHome version to install, e.g. `2026.9.1`                                 |
| `secrets-file`      | `''`      | Secrets file to use instead of dummy secrets, relative to the repository root |
| `secrets`           | `''`      | YAML mapping merged on top of the generated dummy secrets                   |

## How it works

1. **yamllint** runs on every YAML file in `working-directory`.
2. **Secrets** are prepared for `!secret` references.
3. **`esphome config`** runs on every device file.
4. The action fails if any step found an error. All devices are checked before
   it fails, so one run shows every broken device.

### yamllint

Uses `.yamllint`, `.yamllint.yaml` or `.yamllint.yml` from `working-directory`
if present. Otherwise it uses the bundled [`yamllint.yaml`][yamllint-config],
which extends the yamllint defaults with ESPHome conventions:

- no `---` document start required
- line length warning at 120 characters
- `on` / `off` allowed as booleans

### Device files

A device file is any file matching `files` that has a top-level `esphome:`
key. `secrets.yaml` is always excluded. Other matching files, such as include
fragments at the root, are skipped with a notice.

Keep device files in `working-directory` and fragments in subfolders
(`packages/`, `common/`), as the ESPHome dashboard does, and the default glob
works as is.

### Secrets

`esphome config` fails without a `secrets.yaml`, which is usually not
committed. The action picks the first that applies:

1. `secrets-file`, copied to `<working-directory>/secrets.yaml`
2. an existing `<working-directory>/secrets.yaml`
3. generated dummy secrets, one for every `!secret <name>` in
   `working-directory`

Dummy values are chosen to pass ESPHome validation:

| Secret name     | Dummy value                                                   |
| --------------- | ------------------------------------------------------------- |
| contains `key`  | 32 byte base64 string, valid as an `api` encryption key       |
| anything else   | `dummy123`, valid as a wifi SSID and password                 |

If a field needs a different value, override it:

```yaml
- uses: josa42/actions/esphome-lint@main
  with:
    secrets: |
      latitude: 52.52
      static_ip: 192.168.1.50
```

### Annotations

Errors from `esphome config` are attached to the file and line they come from,
including files pulled in through `packages:` or `!include`. YAML syntax errors
also include the column. Errors without location info are attached to the
device file.

## Limitations

- **One error per device per run.** ESPHome stops at the first failing
  component, so fix and push to see the next one.
- **Block level line numbers.** Schema errors point at the first line of the
  failing component, not the exact key. The message names the key.
- **Packages only devices are skipped.** A device that only gets `esphome:`
  through `packages:` has no top-level `esphome:` key.
- **Network access.** Remote `packages:` and `external_components` are fetched
  during validation.
- **Pip cache with `latest`.** The cache key is the version string, so pin
  `esphome-version` for reproducible and fully cached runs.

<!-- internal links -->

[yamllint-config]: ./yamllint.yaml

<!-- external links -->

[esphome]: https://esphome.io
[yamllint]: https://yamllint.readthedocs.io
[build-action]: https://github.com/esphome/build-action
