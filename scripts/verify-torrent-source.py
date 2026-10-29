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
               '44a71bff56c53b4c472c8f8e955064777dfe37a75e901abbcd827f099a0cef49'),
 'torrent.go': ('aa4dc01cb8caed2cfb2196b4cb8ffde92a05ea243de38086cc92d95096f3deeb',
                'a689f4cde811435e0b368b19f9a4695bb845080de0acf3e6ab313f2bceeef70a'),
 'client-tracker-announcer.go': ('7d86f9eecd54d4fb68486b16d9665c9f7e3022b7d4de2dc14733918fed32f3b5',
                                 '804601fe886eb349853e7b0a74370e2d5d26b10653b711adc8912b366724ebb0'),
 'torrent-piece-request-order.go': ('f4ad3c545eb4cecd4e35857170f5b7c32aa71c46713eba0f5d3acfc0fc085423',
                                    'd8512d8103be78ee729de3d8cb0fc86b65aba8f6a848afb333eda0bce2cb4a44'),
 'piece.go': ('915c6126a12c978839d257dff5c07b9967e65ed99416b3ee1b63fe934ef4b812',
              'db2f54cb52ee6fedbd1f00c4866bcef30b149137e2ecb83edb0c074d65756dbd')}
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
print(f"Verified {len(expected)} upstream files; only the six reviewed identity, tracker and inventory source changes")
