"""Prepare CI signing settings without creating an empty release configuration."""

import argparse
import os
from pathlib import Path
import sys


SIGNING_KEYS = {
    "keystore.password": "SIGNING_STORE_PASSWORD",
    "key.alias": "SIGNING_KEY_ALIAS",
    "key.password": "SIGNING_KEY_PASSWORD",
}


def output_configured(configured):
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with Path(output).open("a") as stream:
            stream.write(f"configured={str(configured).lower()}\n")


def properties_value(value):
    encoded = value.encode("utf-16-be")
    return "".join(f"\\u{encoded[i:i + 2].hex()}" for i in range(0, len(encoded), 2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--required", action="store_true")
    args = parser.parse_args()
    values = {key: os.environ.get(name, "") for key, name in SIGNING_KEYS.items()}

    if not any(values.values()) and not args.required:
        if Path("signing.properties").exists():
            sys.exit("Refusing to use existing signing.properties without CI signing secrets.")
        output_configured(False)
        print("::notice::Signing secrets are absent. Build test APKs with the debug key; skip release publication.")
        return

    missing = [SIGNING_KEYS[key] for key, value in values.items() if not value]
    if missing:
        sys.exit("Configure all signing secrets together. Missing: " + ", ".join(missing))
    if not Path("release.keystore").is_file():
        sys.exit("release.keystore is missing.")

    settings = Path("signing.properties")
    with open(settings, "x", encoding="ascii", opener=lambda path, flags: os.open(path, flags, 0o600)) as stream:
        for key, value in values.items():
            stream.write(f"{key}={properties_value(value)}\n")
    output_configured(True)
    print("Release signing settings prepared.")


if __name__ == "__main__":
    main()
