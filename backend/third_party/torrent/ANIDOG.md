# AniDog dependency provenance

This directory contains the complete Go module `github.com/anacrolix/torrent`
v1.61.0, upstream commit `486bcbe758e0cff0f750fcee9c26268a11250d77`.
The upstream LICENSE and source layout are retained. The module checksum is
pinned in the parent `go.sum`.

There is one source change, in `PeerConn.peerSentHaveNone` in `peerconn.go`:
always reconcile the peer's advertised availability before clearing its
inventory. `decPeerPieceAvailability` already handles both the all-pieces set
and individual piece counters. Previously HAVE_ALL followed by HAVE_NONE left
the all-pieces set populated; a later HAVE incremented relative availability,
and closing the connection skipped its decrement. Dropping the torrent or
closing the client then panicked. Assertions remain enabled.

`internal/downloader/providers/embedded/peer_lifecycle_test.go` in the parent
module exercises actual TCP handshakes, inventory messages, removal and client
shutdown in fresh processes, with both classic and mmap storage. The original
source reproduces the failure with the same test.

The dependency owns protocol state. Reimplementing inventory state in the
application, filtering peer messages, swallowing panics, and disabling the fast
extension would leave that responsibility split or hide corrupted state. A
local source replacement instead keeps the correction at its owner, at the
cost of carrying this module until a verified upstream release incorporates
the correction. The latest published upstream version was still v1.61.0 when
this fix was verified; no unreleased upstream changes are included.

Run `python3 scripts/verify-torrent-source.py` from the repository root to
compare every file with the checksum-verified upstream module and allow only
this documented change plus this file. Backend CI runs this check. A future
upgrade must first pass the lifecycle, control, retained-partial recovery and
NAS storage contracts, then remove the replacement and this directory together.
