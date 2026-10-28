# AniDog dependency provenance

This directory contains the complete Go module `github.com/anacrolix/torrent`
v1.61.0, upstream commit `486bcbe758e0cff0f750fcee9c26268a11250d77`.
The upstream LICENSE and source layout are retained. The module checksum is
pinned in the parent `go.sum`.

The inventory correction is in `PeerConn.peerSentHaveNone` in `peerconn.go`:
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
this fix was verified. The additional reviewed changes below are explicit;
unrelated unreleased upstream changes are not included.

Run `python3 scripts/verify-torrent-source.py` from the repository root to
compare every file with the checksum-verified upstream module and allow only
the exact four documented source-file digests plus this file. Backend CI runs this check. A future
upgrade must first pass the lifecycle, control, retained-partial recovery and
NAS storage contracts, then remove the replacement and this directory together.

## v0.4.27: hybrid identity and tracker ownership

`Client.AddTorrentOpt` publishes both supplied v1/v2 aliases under the client
lock before setting metadata or scheduling trackers. The metadata path validates
already-known hashes without filling aliases, so registration must own this
responsibility. Removal already deletes both aliases. No hash is invented when
it is unknown.

`initRegularTrackerAnnounceState` only records successfully registered keys and
ignores dropped owners. The registration-result guard is a narrow backport of
[upstream PR #1079](https://github.com/anacrolix/torrent/pull/1079), merge commit
`7e8d40966b7cc3da5feee16bb8ff6580da48b9a4`. The upstream assertion stays enabled.

The same-hash remove/readd regression also proved that the dispatcher's weak
reference could retain the old owner indefinitely. Live announce requests now
resolve the authoritative registry; weak references serve only detached Stopped
bookkeeping. An in-flight response from an old generation cannot change the new
owner's announce state or deliver peers to it. This avoids splitting protocol
ownership into an application retry or a second task registry.

`tracker_lifecycle_test.go` uses fresh processes, real HTTP trackers and TCP
handshakes with classic/mmap storage. It covers full hybrid metadata, hybrid
magnet identities, removal, readd with another storage directory, a blocked HTTP
response released after replacement, late AddTrackers and shutdown. The original
code panics on hybrid registration; alias registration alone still fails readd.
The source verifier pins before/after SHA256 for all four changed source files
and compares the rest of the complete module byte for byte.
