"""Read release notes from the matching CHANGELOG section."""

import argparse
import re
from pathlib import Path


def release_notes(changelog, tag):
    if not re.fullmatch(r"vnext\.\d+\.\d+\.\d+(?:-rc\.\d+)?", tag):
        raise ValueError(f"Invalid release tag: {tag}")
    sections = {
        heading.split()[0]: body.strip()
        for heading, body in re.findall(
            r"^## ([^\n]+)\n(.*?)(?=^## |\Z)", changelog, re.MULTILINE | re.DOTALL
        )
    }
    for version in (tag, tag.split("-rc.")[0]):
        if version in sections:
            if not sections[version]:
                raise ValueError(f"Empty CHANGELOG section: {version}")
            return sections[version] + "\n"
    raise ValueError(f"No CHANGELOG section for {tag}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag")
    args = parser.parse_args()
    try:
        print(release_notes(Path("CHANGELOG.md").read_text(), args.tag), end="")
    except (OSError, ValueError) as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()
