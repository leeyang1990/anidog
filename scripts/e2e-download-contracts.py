#!/usr/bin/env python3
"""Runs only isolated fixtures; requires Docker, Python libtorrent and a preloaded PostgreSQL image.
Never mounts production state or queries its database. Evidence and fixture media remain for review.
"""

import argparse
import hashlib
import http.server
import json
import os
import secrets
import shutil
import socket
import struct
import subprocess
import threading
import time
import urllib.parse
import urllib.request
from pathlib import Path

import libtorrent as lt

parser = argparse.ArgumentParser(
    description="Real PostgreSQL/BT/media acceptance contracts; Linux Docker host only"
)
parser.add_argument("--image", required=True, help="Exact backend image tag or digest")
parser.add_argument(
    "--workspace", required=True, type=Path, help="New private evidence directory"
)
parser.add_argument(
    "--media-parent",
    required=True,
    type=Path,
    help="Existing media filesystem for an isolated fixture directory",
)
parser.add_argument(
    "--binary",
    type=Path,
    help="Optional candidate Linux binary; omitted for official-image acceptance",
)
parser.add_argument(
    "--retention-probe",
    required=True,
    type=Path,
    help="Linux scheduler test binary built from the same source revision",
)
options = parser.parse_args()
root = options.workspace.resolve()
root.mkdir(mode=0o700, parents=True, exist_ok=False)
fixtures = root / "fixtures"
fixtures.mkdir(mode=0o700)
if options.binary:
    shutil.copy2(options.binary.resolve(strict=True), fixtures / "anidog")
shutil.copy2(options.retention_probe.resolve(strict=True), fixtures / "retention-probe")
run_id = secrets.token_hex(5)
data = options.media_parent.resolve(strict=True) / (".anidog-e2e-" + run_id)
data.mkdir(mode=0o700)
net = "anidog-e2e-" + run_id
pg, app = net + "-postgres", net + "-backend"
runtime = options.image


def run(args, **kw):
    return subprocess.run(
        args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kw
    )


def docker(*args):
    return run(["docker", *args])


def info():
    return json.loads(docker("inspect", app).stdout)[0]


def api(path, body=None, form=None, method=None):
    headers = {}
    if token:
        headers["Authorization"] = "Bearer " + token
    if form is not None:
        raw = urllib.parse.urlencode(form).encode()
        headers["Content-Type"] = "application/x-www-form-urlencoded"
    elif body is not None:
        raw = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    else:
        raw = None
    req = urllib.request.Request(
        api_base + "/api/v1" + path,
        raw,
        headers,
        method=method or ("PUT" if path == "/settings" and body else None),
    )
    with opener.open(req, timeout=15) as r:
        return json.load(r)


token = ""
started = []
session = None
tracker = None
tracker_requests = 0
api_base = ""
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
try:
    docker("network", "create", "--internal", net)
    started.append(("network", net))
    gateway = json.loads(docker("network", "inspect", net).stdout)[0]["IPAM"]["Config"][
        0
    ]["Gateway"]
    session = lt.session(
        {
            "listen_interfaces": gateway + ":0",
            "enable_dht": False,
            "enable_lsd": False,
            "enable_upnp": False,
            "enable_natpmp": False,
        }
    )
    seed_port = session.listen_port()
    assert seed_port > 0, "Seed listener did not bind"

    class Tracker(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            global tracker_requests
            tracker_requests += 1
            body = lt.bencode(
                {
                    b"interval": 2,
                    b"min interval": 1,
                    b"complete": 1,
                    b"incomplete": 0,
                    b"peers": socket.inet_aton(gateway) + struct.pack("!H", seed_port),
                }
            )
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    tracker = http.server.ThreadingHTTPServer((gateway, 0), Tracker)
    threading.Thread(target=tracker.serve_forever, daemon=True).start()

    password = secrets.token_hex(24)
    envfile = root / "e2e-private.env"
    envfile.write_text(
        "POSTGRES_DB=fixture\nPOSTGRES_USER=fixture\nPOSTGRES_PASSWORD="
        + password
        + "\n"
    )
    envfile.chmod(0o600)
    docker(
        "run",
        "-d",
        "--name",
        pg,
        "--network",
        net,
        "--env-file",
        str(envfile),
        "postgres:16-alpine",
    )
    started.append(("container", pg))
    for _ in range(30):
        if (
            subprocess.run(
                ["docker", "exec", pg, "pg_isready", "-U", "fixture"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            ).returncode
            == 0
        ):
            break
        time.sleep(1)
    else:
        raise RuntimeError("isolated PostgreSQL not ready")
    text = "1\n00:00:01,000 --> 00:00:03,000\n这是一份简体字幕，测试下载后的实际文件。\n\n2\n00:01:00,000 --> 00:01:03,000\n我们会检查画质、语言和归档，不会跳过验证。\n"
    (fixtures / "simplified.srt").write_text(text)
    (fixtures / "traditional.srt").write_text(
        "1\n00:00:01,000 --> 00:00:03,000\n這是一份繁體字幕，測試實際檔案。\n"
    )
    cases = [
        ("accepted-v2", "1920x1080", "simplified", "CHS", "zh-Hans"),
        ("accepted", "1920x1080", "simplified", "CHS", "zh-Hans"),
        ("low-resolution", "1280x720", "simplified", "CHS", "zh-Hans"),
        ("traditional", "1920x1080", "traditional", "CHT", "zh-Hant"),
        ("unknown", "1920x1080", None, None, None),
    ]
    for name, size, sub, title, lang in cases:
        args = [
            "ffmpeg",
            "-y",
            "-v",
            "error",
            "-f",
            "lavfi",
            "-i",
            f"color=c=black:s={size}:r=2:d=120",
            "-f",
            "lavfi",
            "-i",
            "sine=frequency=1000:sample_rate=48000",
        ]
        if sub:
            args += ["-i", f"/fixtures/{sub}.srt"]
        args += ["-map", "0:v", "-map", "1:a"]
        if sub:
            args += [
                "-map",
                "2:s",
                "-c:s",
                "srt",
                "-metadata:s:s:0",
                f"title={title}",
                "-metadata:s:s:0",
                f"language={lang}",
            ]
        args += [
            "-t",
            "120",
            "-c:v",
            "libx264",
            "-preset",
            "ultrafast",
            "-c:a",
            "aac",
            "-b:a",
            "128k",
            f"/fixtures/{name}.mkv",
        ]
        # The runtime includes the same tools as production; generate fixtures in a
        # separate container with no production state, database or media mounted.
        docker(
            "run",
            "--rm",
            "--network",
            net,
            "-v",
            str(fixtures) + ":/fixtures",
            "--entrypoint",
            args[0],
            runtime,
            *args[1:],
        )
        fs = lt.file_storage()
        lt.add_files(fs, str(fixtures / (name + ".mkv")))
        ct = (
            lt.create_torrent(fs, 32 * 1024, lt.create_torrent.v2_only)
            if name == "accepted-v2"
            else lt.create_torrent(fs, 32 * 1024)
        )
        ct.add_tracker(
            "http://" + gateway + ":" + str(tracker.server_port) + "/announce"
        )
        lt.set_piece_hashes(ct, str(fixtures))
        (fixtures / (name + ".torrent")).write_bytes(lt.bencode(ct.generate()))
    envfile.write_text(
        "DATABASE_URL=postgres://fixture:"
        + password
        + "@"
        + pg
        + ":5432/fixture?sslmode=disable\nSECRET_KEY="
        + secrets.token_hex(32)
        + "\nMEDIA_ROOT=/downloads\nBT_STATE_DIR=/bt-state\nBT_LISTEN_PORT=6881\nENABLE_SCHEDULER=true\nENABLE_NOTIFICATIONS=false\nENABLE_DEFAULT_RULES=false\nHTTP_PROXY=\nTORRENT_STORAGE_DEFAULT_FILE_IO=classic\nVERIFY_DOWNLOADED_MEDIA=true\n"
    )
    (root / ("e2e-state-" + str(os.getpid()))).mkdir(mode=0o700)
    docker(
        "run",
        "-d",
        "--name",
        app,
        "--network",
        net,
        "--env-file",
        str(envfile),
        "-v",
        str(fixtures) + ":/fixtures:ro",
        "-v",
        str(data) + ":/downloads",
        "-v",
        str(root / ("e2e-state-" + str(os.getpid()))) + ":/bt-state",
        *(["--entrypoint", "/fixtures/anidog"] if options.binary else []),
        runtime,
    )
    started.append(("container", app))
    address = next(iter(info()["NetworkSettings"]["Networks"].values()))["IPAddress"]
    api_base = "http://" + address + ":8088"
    for _ in range(60):
        try:
            with opener.open(api_base + "/healthcheck", timeout=2):
                break
        except Exception:
            time.sleep(1)
    else:
        raise RuntimeError("isolated backend not ready")
    api("/auth/register", {"username": "fixture", "password": password})
    token = api("/auth/login", form={"username": "fixture", "password": password})[
        "access_token"
    ]
    api(
        "/settings",
        {
            "download.quality": "1080p",
            "download.languages": '["simplified"]',
            "download.allow_preference_fallback": "false",
            "media.min_free_gb": "0",
            "media.max_used_percent": "0",
        },
    )
    gateway = json.loads(docker("network", "inspect", net).stdout)[0]["IPAM"]["Config"][
        0
    ]["Gateway"]
    address = next(iter(info()["NetworkSettings"]["Networks"].values()))["IPAddress"]
    probe = docker(
        "run",
        "--rm",
        "--network",
        net,
        "--env-file",
        str(envfile),
        "-e",
        "ANIDOG_POSTGRES_CONTRACT=1",
        "-v",
        str(fixtures) + ":/fixtures:ro",
        "--entrypoint",
        "/fixtures/retention-probe",
        runtime,
        "-test.run=^TestPostgresEvidenceRetention$",
        "-test.v",
        "-test.timeout=60s",
    )
    (root / "postgres-retention-contracts.log").write_bytes(probe.stdout + probe.stderr)
    v2_restart_receipt = None
    targets = []
    for name, size, sub, title, lang in cases:
        anime = api(
            "/anime/",
            {
                "title": "E2E " + name,
                "year": 2026,
                "season": 1,
                "episode_count": 1,
                "is_subscribed": False,
            },
        )
        api(
            f"/anime/{anime['id']}/episodes",
            {"episode_number": 1, "duration_seconds": 120},
        )
        ti = lt.torrent_info(str(fixtures / (name + ".torrent")))
        params = lt.add_torrent_params()
        params.ti = ti
        params.save_path = str(fixtures)
        params.flags &= ~(lt.torrent_flags.paused | lt.torrent_flags.auto_managed)
        if name == "accepted-v2":
            params.flags |= lt.torrent_flags.paused
        handle = session.add_torrent(params)
        row = api(
            "/downloads/",
            {
                "name": "[CHS][1080p] E2E " + name,
                "url": "/fixtures/" + name + ".torrent",
                "download_type": "torrent",
                "source": "bt",
                "anime_id": anime["id"],
                "episode_number": 1,
            },
        )
        if name == "accepted-v2":
            expected_v2 = str(ti.info_hashes().v2).upper()
            for _ in range(30):
                row = api("/downloads/" + str(row["id"]))
                if row.get("info_hash"):
                    break
                time.sleep(1)
            assert row.get("info_hash") == expected_v2 and len(expected_v2) == 64
            attempt = row["torrent_id"]
            assert attempt != expected_v2
            paused = api(f"/downloads/{row['id']}/pause", method="POST")
            assert paused["status"] == "paused"
            docker("restart", app)
            for _ in range(60):
                try:
                    row = api("/downloads/" + str(row["id"]))
                    break
                except Exception:
                    time.sleep(1)
            else:
                raise RuntimeError("paused v2 task did not recover")
            assert row["status"] == "paused" and row["info_hash"] == expected_v2
            assert row["torrent_id"] == attempt
            handle.resume()
            api(f"/downloads/{row['id']}/resume", method="POST")
            v2_restart_receipt = {
                "id": row["id"],
                "full_v2": expected_v2,
                "attempt_id": attempt,
                "paused_restart_preserved": True,
            }
        targets.append(
            {
                "name": name,
                "anime_id": anime["id"],
                "id": row["id"],
                "handle": handle,
                "done": False,
            }
        )
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        for target in targets:
            if target["done"]:
                continue
            # Peer discovery uses the real local tracker, in both transfer directions.
            row = api("/downloads/" + str(target["id"]))
            if row["status"] in ["completed", "failed"]:
                target["row"] = row
                target["done"] = True
        (root / "e2e-peer-progress.json").write_text(
            json.dumps(
                [
                    {
                        "case": t["name"],
                        "seed_progress": t["handle"].status().progress,
                        "peers": t["handle"].status().num_peers,
                        "upload_payload_rate": t["handle"].status().upload_payload_rate,
                        "seed_error": t["handle"].status().errc.message(),
                    }
                    for t in targets
                ],
                indent=2,
            )
        )
        if all(t["done"] for t in targets):
            break
        time.sleep(2)
    assert all(t["done"] for t in targets), "transfer/validation did not settle"
    results = []
    for t in targets:
        row = t["row"]
        eps = api(f"/anime/{t['anime_id']}/episodes")
        assert len(eps) == 1
        ep = eps[0]
        if t["name"] in ["accepted", "accepted-v2"]:
            assert row["status"] == "completed" and ep["downloaded"]
            path = data / row["file_path"].removeprefix("/downloads/")
            assert (
                path.is_file()
                and hashlib.sha256(path.read_bytes()).digest()
                == hashlib.sha256(
                    (fixtures / (t["name"] + ".mkv")).read_bytes()
                ).digest()
            )
            assert ep["file_path"] == row["file_path"]
        else:
            assert row["status"] == "failed" and not ep["downloaded"], (
                t["name"],
                row["status"],
            )
            assert row["failure_code"] == (
                "subtitle_unknown" if t["name"] == "unknown" else "preference_mismatch"
            ), (t["name"], row["failure_code"])
            if t["name"] == "unknown":
                assert row["validation_pending"]
                assert (data / row["file_path"].removeprefix("/downloads/")).is_file()
        results.append(
            {
                "case": t["name"],
                "id": row["id"],
                "status": row["status"],
                "failure_code": row.get("failure_code"),
                "file_present": bool(
                    row.get("file_path")
                    and (data / row["file_path"].removeprefix("/downloads/")).is_file()
                ),
                "episode_downloaded": ep["downloaded"],
            }
        )
    # Simulate an explicitly imported legacy 720p asset in the isolated library.
    # The normal post-archive audit, not SQL, establishes its quality debt.
    upgrade_anime = api(
        "/anime/",
        {
            "title": "E2E verified replacement",
            "year": 2026,
            "season": 1,
            "episode_count": 1,
            "status": "finished",
            "is_subscribed": False,
        },
    )
    api(
        f"/anime/{upgrade_anime['id']}/episodes",
        {"episode_number": 1, "duration_seconds": 120},
    )
    seed_by_name = {t["name"]: t["handle"] for t in targets}

    def transfer_case(name, source):
        row = api(
            "/downloads/",
            {
                "name": "E2E " + name,
                "url": "/fixtures/" + name + ".torrent",
                "download_type": "torrent",
                "source": source,
                "anime_id": upgrade_anime["id"],
                "episode_number": 1,
            },
        )
        until = time.monotonic() + 240
        while time.monotonic() < until:
            row = api("/downloads/" + str(row["id"]))
            st = seed_by_name[name].status()
            details = {
                "name": name,
                "row": {
                    k: row.get(k)
                    for k in [
                        "id",
                        "status",
                        "progress",
                        "info_hash",
                        "torrent_id",
                        "connected_seeds",
                        "availability",
                    ]
                },
                "seed": {
                    "progress": st.progress,
                    "paused": st.paused,
                    "state": str(st.state),
                    "peers": st.num_peers,
                    "error": st.errc.message(),
                },
                "events": [str(a) for a in session.pop_alerts()][-40:],
            }
            with (root / "replacement-peer-observations.jsonl").open("a") as out:
                out.write(json.dumps(details) + "\n")
            if row["status"] in ["completed", "failed", "paused", "superseded"]:
                return row
            time.sleep(1)
        raise RuntimeError("replacement transfer did not settle")

    legacy = transfer_case("low-resolution", "manual")
    assert legacy["status"] == "completed", legacy.get("last_error")
    for _ in range(40):
        old_episode = api(f"/anime/{upgrade_anime['id']}/episodes")[0]
        if old_episode["quality_status"] == "upgrade_needed":
            break
        time.sleep(1)
    assert old_episode["quality_status"] == "upgrade_needed", (
        "normal quality audit did not identify 720p"
    )
    upgraded = transfer_case("accepted", "bt")
    assert upgraded["status"] == "completed", upgraded.get("last_error")
    legacy_after = api("/downloads/" + str(legacy["id"]))
    new_episode = api(f"/anime/{upgrade_anime['id']}/episodes")[0]
    assert (
        legacy_after["status"] == "superseded"
        and new_episode["downloaded"]
        and new_episode["quality_status"] == "meets_target"
    )
    assert (
        new_episode["download_id"] == upgraded["torrent_id"]
        and new_episode["file_path"] == upgraded["file_path"]
    )
    backup = data / legacy_after["file_path"].removeprefix("/downloads/")
    promoted = data / upgraded["file_path"].removeprefix("/downloads/")
    assert (
        hashlib.sha256(backup.read_bytes()).digest()
        == hashlib.sha256((fixtures / "low-resolution.mkv").read_bytes()).digest()
    )
    assert (
        hashlib.sha256(promoted.read_bytes()).digest()
        == hashlib.sha256((fixtures / "accepted.mkv").read_bytes()).digest()
    )
    replacement = {
        "legacy_id": legacy["id"],
        "upgraded_id": upgraded["id"],
        "quality_debt_from_normal_audit": True,
        "old_sha256": hashlib.sha256(backup.read_bytes()).hexdigest(),
        "new_sha256": hashlib.sha256(promoted.read_bytes()).hexdigest(),
        "backup_preserved": True,
        "real_normal_api_and_bt": True,
    }
    expected_settings = {
        "download.quality": "1080p",
        "download.languages": '["simplified"]',
        "download.allow_preference_fallback": "false",
        "media.min_free_gb": "0",
        "media.max_used_percent": "0",
    }
    for key, value in expected_settings.items():
        assert api("/settings")[key] == value, (key, "save")
    before = api("/downloads/current?page_size=100")
    docker("restart", app)
    for _ in range(60):
        try:
            after = api("/downloads/current?page_size=100")
            break
        except Exception:
            time.sleep(1)
    else:
        raise RuntimeError("restart did not recover")
    assert after["counts"] == before["counts"]
    for key, value in expected_settings.items():
        assert api("/settings")[key] == value, (key, "restart")

    for t in targets:
        assert api("/downloads/" + str(t["id"]))["status"] == t["row"]["status"]
    logs = docker("logs", app).stdout.decode(errors="replace")
    # Docker logs splits stderr and stdout; retain both independently below.
    assert info()["RestartCount"] == 0
    assert (
        hashlib.sha256(backup.read_bytes()).hexdigest() == replacement["old_sha256"]
        and hashlib.sha256(promoted.read_bytes()).hexdigest()
        == replacement["new_sha256"]
    )
    assert tracker_requests > 0
    result = {
        "normal_tracker_discovery": True,
        "pure_v2_restart_and_transfer": v2_restart_receipt,
        "postgres_retention_and_concurrent_fk": True,
        "tracker_requests": tracker_requests,
        "single_episode_upgrade": replacement,
        "candidate_binary": bool(options.binary),
        "version": api("/system/info")["version"],
        "real_postgresql": True,
        "real_classic_bt_transfer": True,
        "isolated_nas_directory": str(data),
        "production_database_mounted": False,
        "strict_requirements": True,
        "real_archive_sha256_matches_seed": True,
        "restart_preserves_outcomes": True,
        "zero_capacity_settings_persist": True,
        "cases": results,
    }
    (root / "e2e-result.json").write_text(
        json.dumps(result, ensure_ascii=False, indent=2)
    )
    print(json.dumps(result, ensure_ascii=False), flush=True)
finally:
    if session:
        session.pause()
    if tracker:
        tracker.shutdown()
        tracker.server_close()
    if any(n == app for k, n in started):
        result = subprocess.run(
            ["docker", "logs", app], stdout=subprocess.PIPE, stderr=subprocess.STDOUT
        )
        (root / "e2e-backend.log").write_bytes(result.stdout)
    for kind, name in reversed(started):
        subprocess.run(
            ["docker", "rm", "-f", "-v", name]
            if kind == "container"
            else ["docker", "network", "rm", name],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    if "envfile" in globals():
        envfile.unlink(missing_ok=True)
