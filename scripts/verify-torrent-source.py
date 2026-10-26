#!/usr/bin/env python3
"""Verify the local dependency contains exactly its documented source change."""

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
original = b"\tif !cn.peerSentHaveAll {\n\t\tcn.t.decPeerPieceAvailability(&cn.Peer)\n\t}\n"
corrected = (
    b"\t// Reconcile both inventory representations before clearing the peer's\n"
    b"\t// advertised pieces: HAVE_ALL uses connsWithAllPieces, whereas BITFIELD\n"
    b"\t// and HAVE use per-piece relative availability.\n"
    b"\tcn.t.decPeerPieceAvailability(&cn.Peer)\n"
)
expected = {p.relative_to(upstream): p.read_bytes() for p in upstream.rglob("*") if p.is_file()}
actual = {p.relative_to(local): p.read_bytes() for p in local.rglob("*") if p.is_file()}
source = Path("peerconn.go")
if expected[source].count(original) != 1:
    raise SystemExit("Upstream inventory function no longer matches the reviewed change")
expected[source] = expected[source].replace(original, corrected, 1)
if not actual.pop(Path("ANIDOG.md"), None):
    raise SystemExit("Dependency provenance is missing")
differences = sorted(str(p) for p in expected.keys() | actual.keys() if expected.get(p) != actual.get(p))
if differences:
    raise SystemExit("Undocumented dependency changes: " + ", ".join(differences))
print(f"Verified {len(expected)} upstream files; exactly one reviewed inventory correction")
