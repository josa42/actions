"""Run `esphome config` on every device file and annotate failures.

Device files are the FILES globs (relative to the working directory) that have
a top-level `esphome:` key. Devices that only get `esphome:` through
`packages:` are skipped too.
"""

import os
import re
import subprocess
import sys
from pathlib import Path

ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
ESPHOME_KEY_RE = re.compile(r"^esphome:", re.MULTILINE)
# Schema errors: "wifi: [source packages/wifi.yaml:6]"
SOURCE_RE = re.compile(r"^\S.*\[source (.+?):(\d+)\]")
# YAML syntax errors: 'in "dev.yaml", line 7, column 13'
YAML_RE = re.compile(r'in "(.+?)", line (\d+), column (\d+)')

WORKSPACE = Path(os.environ.get("GITHUB_WORKSPACE", ".")).resolve()


def workspace_path(path):
    """Path relative to the repo root, or None if it lies outside of it."""
    rel = os.path.relpath(Path(path).resolve(), WORKSPACE)
    return None if rel.startswith("..") else rel


def escape(text):
    return text.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")


def annotate(device, message, file=None, line=None, col=None):
    props = f"file={workspace_path(file) or workspace_path(device)}"
    if line and workspace_path(file):
        props += f",line={line}"
        if col:
            props += f",col={col}"
    print(f"::error {props}::{escape(message.strip())}")


def find_devices(globs):
    files = set()
    for pattern in globs:
        files.update(p for p in Path(".").glob(pattern) if p.is_file())

    devices = []
    for path in sorted(files):
        if path.name == "secrets.yaml":
            continue
        if not ESPHOME_KEY_RE.search(path.read_text(errors="replace")):
            print(f"::notice file={workspace_path(path)}::Skipped, no top-level esphome key")
            continue
        devices.append(path)
    return devices


def report(device, output):
    lines = output.splitlines()

    if "Failed config" in lines:
        blocks = []
        for line in lines[lines.index("Failed config") + 1 :]:
            match = SOURCE_RE.match(line)
            if match:
                blocks.append((match.group(1), match.group(2), [line]))
            elif blocks:
                blocks[-1][2].append(line)
        for file, line, block in blocks:
            annotate(device, "\n".join(block), file, line)
        if blocks:
            return

    match = YAML_RE.search(output)
    if match:
        message = output[output.find("ERROR") :] if "ERROR" in output else output
        annotate(device, message, *match.groups())
        return

    errors = [l for l in lines if l.startswith("ERROR")]
    annotate(device, "\n".join(errors) or output)


def main():
    globs = [g.strip() for g in os.environ.get("FILES", "*.yaml").splitlines() if g.strip()]
    devices = find_devices(globs)
    if not devices:
        print("::warning::No ESPHome device files found")
        return

    failed = []
    for device in devices:
        print(f"::group::{device}")
        result = subprocess.run(
            ["esphome", "config", str(device)],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        output = ANSI_RE.sub("", result.stdout)
        print(output)
        print("::endgroup::")
        if result.returncode != 0:
            failed.append(device)
            report(device, output)

    print(f"{len(devices) - len(failed)}/{len(devices)} device configs valid")
    if failed:
        print("Failed: " + ", ".join(str(d) for d in failed))
        sys.exit(1)


if __name__ == "__main__":
    main()
