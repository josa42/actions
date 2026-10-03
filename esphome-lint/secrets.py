"""Provide a secrets.yaml for `esphome config`.

Uses an existing secrets.yaml or SECRETS_FILE as is. Otherwise generates a
dummy value for every `!secret <key>` reference, merged with SECRETS.
"""

import os
import re
import shutil
import sys
from pathlib import Path

import yaml

# Valid for api/encryption keys: base64 that decodes to 32 bytes.
DUMMY_KEY = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
# Fits wifi passwords (8-63 chars) and ssids (max 32 chars).
DUMMY_VALUE = "dummy123"

SECRET_RE = re.compile(r"!secret\s+([\w.-]+)")


def main():
    target = Path("secrets.yaml")
    secrets_file = os.environ.get("SECRETS_FILE", "")

    if secrets_file:
        source = Path(os.environ["GITHUB_WORKSPACE"], secrets_file)
        if source.resolve() != target.resolve():
            shutil.copyfile(source, target)
        print(f"Using secrets from {secrets_file}")
        return

    if target.exists():
        print("Using existing secrets.yaml")
        return

    keys = set()
    for path in [*Path(".").rglob("*.yaml"), *Path(".").rglob("*.yml")]:
        keys.update(SECRET_RE.findall(path.read_text(errors="replace")))

    secrets = {k: DUMMY_KEY if "key" in k.lower() else DUMMY_VALUE for k in sorted(keys)}

    overrides = yaml.safe_load(os.environ.get("SECRETS", "")) or {}
    if not isinstance(overrides, dict):
        print("::error::The secrets input must be a YAML mapping")
        sys.exit(1)
    secrets.update(overrides)

    target.write_text(yaml.safe_dump(secrets))
    print(f"Generated dummy secrets.yaml with {len(secrets)} keys")


if __name__ == "__main__":
    main()
