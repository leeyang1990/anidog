"""Normalize Ubuntu's official archive in all active APT source files."""

from pathlib import Path
import re
import sys

ARCHIVE = re.compile(r"https?://(?:azure\.archive\.ubuntu\.com|archive\.ubuntu\.com)(?=/ubuntu(?:/|\s|$))")


def normalize(root: Path) -> list[Path]:
    candidates = [root / "sources.list"]
    directory = root / "sources.list.d"
    if directory.is_dir():
        candidates.extend(sorted(p for p in directory.iterdir() if p.suffix in {".list", ".sources"}))
    changed = []
    for path in candidates:
        if not path.is_file():
            continue
        original = path.read_text()
        updated = ARCHIVE.sub("https://archive.ubuntu.com", original)
        if updated != original:
            path.write_text(updated)
            changed.append(path)
    return changed


if __name__ == "__main__":
    for path in normalize(Path(sys.argv[1]) if len(sys.argv) > 1 else Path("/etc/apt")):
        print(f"Normalized Ubuntu archive in {path}")
