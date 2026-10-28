#!/usr/bin/env python3
"""Verify the local dependency contains exactly its documented source change."""

import hashlib
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
backend = root / "backend"
module = json.loads(subprocess.check_output(
    ["go", "mod", "download", "-json", "github.com/anacrolix/torrent@v1.61.0"],
    cwd=backend, text=True,
))
upstream = Path(module["Dir"])
local = backend / "third_party" / "torrent"
expected = {p.relative_to(upstream): p.read_bytes() for p in upstream.rglob("*") if p.is_file()}
actual = {p.relative_to(local): p.read_bytes() for p in local.rglob("*") if p.is_file()}
# Exact reviewed source digests; changes and provenance are documented in ANIDOG.md.
reviewed = {'peerconn.go': ('f9ff3c53f7d20d504c57eabbed04720af374f8dd00a15a6d584eb3d753bb51b6',
                 '998100eb0d212eb0ab4d246250f9d573668f3ce312d906ba752c6a3c69e9f03b'),
 'client.go': ('6d8b777108fbd0826879fa2fab00b371c027efb6ae143227dfaae5fbe09ba470',
               '54c088586f773811faf545694d00700266c095d5fc049bfdeeccb16dd0c6b58b'),
 'torrent.go': ('aa4dc01cb8caed2cfb2196b4cb8ffde92a05ea243de38086cc92d95096f3deeb',
                'f15ab03a12e0aeb9daf10cd16508488827d6c264fe75e9d61dfbf047a2c821bb'),
 'client-tracker-announcer.go': ('7d86f9eecd54d4fb68486b16d9665c9f7e3022b7d4de2dc14733918fed32f3b5',
                                 '804601fe886eb349853e7b0a74370e2d5d26b10653b711adc8912b366724ebb0')}
for name, (before, after) in reviewed.items():
    source = Path(name)
    if hashlib.sha256(expected[source]).hexdigest() != before:
        raise SystemExit("Upstream source changed: " + name)
    if hashlib.sha256(actual[source]).hexdigest() != after:
        raise SystemExit("Unreviewed local source change: " + name)
    expected[source] = actual[source]
if not actual.pop(Path("ANIDOG.md"), None):
    raise SystemExit("Dependency provenance is missing")
differences = sorted(str(p) for p in expected.keys() | actual.keys() if expected.get(p) != actual.get(p))
if differences:
    raise SystemExit("Undocumented dependency changes: " + ", ".join(differences))
print(f"Verified {len(expected)} upstream files; only the four reviewed identity, tracker and inventory source changes")
