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
the exact six documented source-file digests plus this file. Backend CI runs this check. A future
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

## v0.4.28: pure v2 registry entry

`Client.AddTorrentOpt` selects a v2 short registry key when the v1 hash is
absent, while preserving the absent v1 field in `opts`. A valid BEP 52 torrent
can therefore reach the existing v2 metadata, piece hashing and wire protocol
implementation without tripping the v1-only entry assertion. The nonzero
registry assertion stays enabled. The application validates its input identity
before calling this dependency and keeps the full v2 receipt separately from
the short wire key; it never synthesizes a v1 identity from a v2 prefix.

Fresh-process provider tests exercise pure v2 file and magnet submission, real
peer transfer, full receipt snapshots, controls, cached metadata recovery,
removal, and hybrid metadata discovery after a v2-only magnet. Independent
libtorrent-generated v2 media additionally passes the HTTP/API/PostgreSQL/NAS
acceptance harness with no replacement of the production binary.

Multi-piece pure v2 additionally exposed two existing initialization invariants.
`onSetInfo` batches piece initialization before publishing pending-work triggers.
The request-order invariant compares data requests plus pieces still needing v2
hashes, matching `updatePendingPieces` semantics. `Piece.setV2Hash` reconciles
request priority even when disk completion is unchanged, before any completion
callback schedules peers. The assertion remains active for both work sets; the
existing v1 pause/control regression continues to exercise its original strict
condition. Multi-piece fresh-process fixtures reproduce the original failure
and verify transfer plus retained-byte recovery. The source verifier now pins
six changed files; the rest of the complete upstream module remains identical.

Outgoing handshakes snapshot both identities and the canonical short key under
the client read lock before network I/O. Hybrid metadata discovery may publish
the v1 alias concurrently; encryption, reserved bits and response validation
must consume one consistent snapshot rather than race on mutable options.
Fresh-process v2-magnet regressions run under the race detector as well.
