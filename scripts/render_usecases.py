from __future__ import annotations

import argparse
import difflib
import re
from pathlib import Path
from typing import NamedTuple

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SOURCE_DIR = REPO_ROOT / "docs" / "usecases"
DEFAULT_SOURCE_PATTERN = "*.sh"
DEFAULT_MARKDOWN_DIR = REPO_ROOT / "docs" / "generated" / "usecases"
GENERATOR_ID = "scripts/render_usecases.py"
GENERATED_MARKDOWN_PATTERN = "*.md"

_DIRECTIVE_RE = re.compile(r"^#\s*([a-z][a-z0-9_-]*):\s*(.*)$")
_TITLE_WORDS = {
    "api": "API",
    "bob": "Bob",
    "http": "HTTP",
    "json": "JSON",
    "stac": "STAC",
}


class Markdown(NamedTuple):
    text: str


class Command(NamedTuple):
    text: str


class UsecaseSource(NamedTuple):
    path: Path
    slug: str
    title: str
    order: int
    events: tuple[Markdown | Command, ...]


class GeneratedArtifact(NamedTuple):
    source: Path
    markdown: Path


def parse_usecase_shell(path: Path) -> UsecaseSource:
    title = _title_from_slug(path.stem)
    order = 1000
    events: list[Markdown | Command] = []
    command_lines: list[str] = []

    def flush_command() -> None:
        if command_lines:
            events.append(Command("\n".join(command_lines).strip()))
            command_lines.clear()

    for raw_line in path.read_text(encoding="utf-8").splitlines():
        stripped = raw_line.strip()
        if not stripped:
            if command_lines:
                command_lines.append(raw_line)
                continue
            if events and isinstance(events[-1], Markdown) and events[-1].text:
                events.append(Markdown(""))
            continue
        if stripped.startswith("#!") or stripped == "set -euo pipefail":
            flush_command()
            continue
        if stripped.startswith("#"):
            flush_command()
            match = _DIRECTIVE_RE.match(stripped)
            if match:
                key, value = match.groups()
                if key == "title":
                    title = value
                    continue
                if key == "order":
                    order = int(value)
                    continue
                if key in {"description", "mark"}:
                    continue
            comment = stripped.removeprefix("#").strip()
            if comment:
                events.append(Markdown(comment))
            continue
        command_lines.append(raw_line)
    flush_command()

    return UsecaseSource(
        path=path,
        slug=path.stem,
        title=title,
        order=order,
        events=tuple(events),
    )


def markdown_from_usecase(usecase: UsecaseSource) -> str:
    lines = [
        f"# {usecase.title}",
        "",
        f"<!-- Generated from `{_display_path(usecase.path)}` by {GENERATOR_ID}; do not edit by hand. -->",
        "",
    ]
    markdown_lines: list[str] = []

    def flush_markdown() -> None:
        if markdown_lines:
            lines.extend(markdown_lines)
            lines.append("")
            markdown_lines.clear()

    for event in usecase.events:
        if isinstance(event, Markdown):
            markdown_lines.append(event.text)
            continue
        flush_markdown()
        lines.extend(["```bash", event.text, "```", ""])
    flush_markdown()
    return "\n".join(lines).rstrip() + "\n"


def generate_usecase_artifacts(
    *,
    source_dir: Path = DEFAULT_SOURCE_DIR,
    source_pattern: str = DEFAULT_SOURCE_PATTERN,
    markdown_dir: Path = DEFAULT_MARKDOWN_DIR,
    check: bool = False,
) -> list[GeneratedArtifact]:
    sources_and_usecases = [
        (source, parse_usecase_shell(source))
        for source in sorted(source_dir.glob(source_pattern))
    ]
    sources_and_usecases.sort(key=lambda item: (item[1].order, item[0].name))
    generated_markdown: list[Path] = []
    artifacts: list[GeneratedArtifact] = []

    for source, usecase in sources_and_usecases:
        markdown_path = markdown_dir / f"{usecase.slug}.md"
        _write_or_check(markdown_path, markdown_from_usecase(usecase), check=check)
        generated_markdown.append(markdown_path)
        artifacts.append(GeneratedArtifact(source=source, markdown=markdown_path))

    if not check:
        _prune_generated_markdown(markdown_dir, keep=generated_markdown)
    return artifacts


def _write_or_check(path: Path, text: str, *, check: bool) -> None:
    if check:
        existing = path.read_text(encoding="utf-8") if path.exists() else ""
        if existing != text:
            diff = "".join(
                difflib.unified_diff(
                    existing.splitlines(keepends=True),
                    text.splitlines(keepends=True),
                    fromfile=str(path),
                    tofile=f"{path} (expected)",
                )
            )
            raise SystemExit(f"{path} is out of date\n{diff}")
        return

    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _prune_generated_markdown(markdown_dir: Path, *, keep: list[Path]) -> None:
    if not markdown_dir.exists():
        return
    keep_set = {path.resolve() for path in keep}
    for path in markdown_dir.glob(GENERATED_MARKDOWN_PATTERN):
        if path.resolve() not in keep_set:
            path.unlink()


def _title_from_slug(slug: str) -> str:
    words = []
    for word in slug.split("-"):
        words.append(_TITLE_WORDS.get(word, word.capitalize()))
    return " ".join(words)


def _display_path(path: Path) -> str:
    try:
        return path.resolve().relative_to(REPO_ROOT).as_posix()
    except ValueError:
        return path.as_posix()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-dir", type=Path, default=DEFAULT_SOURCE_DIR)
    parser.add_argument("--source-pattern", default=DEFAULT_SOURCE_PATTERN)
    parser.add_argument("--markdown-dir", type=Path, default=DEFAULT_MARKDOWN_DIR)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()

    artifacts = generate_usecase_artifacts(
        source_dir=args.source_dir,
        source_pattern=args.source_pattern,
        markdown_dir=args.markdown_dir,
        check=args.check,
    )
    for artifact in artifacts:
        print(artifact.markdown)


if __name__ == "__main__":
    main()
