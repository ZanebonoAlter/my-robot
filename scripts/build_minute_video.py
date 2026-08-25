#!/usr/bin/env python3
"""Build the 1-minute competition video from TTS segments + still shots.

Reads artifacts/competition-video/tts/manifest.json (produced by mimo_tts.py),
generates per-shot Ken Burns motion, xfade transitions, an ASS subtitle track
burned in KaiTi style, and encodes the final MP4.

Usage:
    python3 scripts/build_minute_video.py            # artifacts under repo root
    python3 scripts/build_minute_video.py --dry-run  # print ffmpeg plan only
"""
from __future__ import annotations

import argparse
import json
import math
import os
import shutil
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

FPS = 30
XFADE = 0.5   # crossfade seconds between shots
LEAD = 0.55   # silence before narration starts within a shot
TAIL = 0.85   # breathing room after narration ends

PAPER = "0xfaf7f2"  # pad colour, matches deck

# (segment_id, image path from repo root, motion)
SHOTS = [
    ("01-hook",       "docs/experience/competition-ppt/shots/s2.png",              "zoom-in"),
    ("02-solution",   "docs/experience/competition-ppt/shots/s4.png",              "zoom-in"),
    ("03-coldstart",  "img/1.4.0/board-upgrade-suggestions.png",                   "pan-lr"),
    ("04-explain",    "img/product-video-v2/03-match-detail.jpg",                  "zoom-in-strong"),
    ("05-daily",      "img/1.4.0/daily-report.png",                                "zoom-in"),
    ("06-timeline",   "img/1.4.0/overview.png",                                    "pan-lr"),
    ("07-agent",      "img/1.4.0/data-enrichment-causal-report.png",               "pan-ud"),
    ("08-closing",    "docs/experience/competition-ppt/shots/s15.png",             "still"),
]
DARK_SHOTS = {"08-closing"}  # paper-coloured subtitles on dark slides


def ass_time(t: float) -> str:
    h = int(t // 3600); m = int(t % 3600 // 60); s = t % 60
    return f"{h}:{m:02d}:{s:05.2f}"


def zoompan_expr(motion: str, frames: int) -> str:
    D = frames
    cx, cy = "iw/2-(iw/zoom)/2", "ih/2-(ih/zoom)/2"
    if motion == "zoom-in":
        return f"z='1+0.08*on/{D}':x='{cx}':y='{cy}'"
    if motion == "zoom-in-strong":
        return f"z='1+0.13*on/{D}':x='{cx}':y='{cy}'"
    if motion == "pan-lr":
        return f"z='1.10':x='(iw-iw/zoom)*on/{D}':y='{cy}'"
    if motion == "pan-ud":
        return f"z='1.14':x='{cx}':y='(ih-ih/zoom)*on/{D}'"
    return "z='1.0'"


def build_ass(events: list[dict], path: Path) -> None:
    header = """[Script Info]
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
WrapStyle: 0
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Light,KaiTi,44,&H00202020,&H00FFFFFF,&H50F2F7FA,&H00000000,0,0,0,0,100,100,0,0,3,5,0,2,60,60,96,1
Style: Dark,KaiTi,44,&H00F2F7FA,&H00FFFFFF,&H00202020,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,60,60,96,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
"""
    lines = [header]
    for ev in events:
        style = "Dark" if ev["id"] in DARK_SHOTS else "Light"
        text = ev["text"].replace("\n", "\\N")
        lines.append(
            f"Dialogue: 0,{ass_time(ev['start'])},{ass_time(ev['end'])},{style},,0,0,0,,{text}\n"
        )
    path.write_text("".join(lines), encoding="utf-8")


def find_ffmpeg() -> str:
    win = Path("/mnt/d/tool/ffmpeg/ffmpeg.exe")
    if win.exists():
        return str(win)
    found = shutil.which("ffmpeg")
    if found:
        return found
    sys.exit("ffmpeg not found (looked at /mnt/d/tool/ffmpeg/ffmpeg.exe and PATH)")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--tts-dir", default="artifacts/competition-video/tts")
    ap.add_argument("--out", default="artifacts/competition-video")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    tts_dir = (REPO / args.tts_dir).resolve()
    out_dir = (REPO / args.out).resolve()
    build_dir = out_dir / "build"
    manifest = json.loads((tts_dir / "manifest.json").read_text(encoding="utf-8"))
    dur = {s["id"]: s["duration_seconds"] for s in manifest["segments"]}
    text = {s["id"]: s["text"] for s in manifest["segments"]}
    audio_file = {s["id"]: s.get("file", f"{s['id']}.wav") for s in manifest["segments"]}
    missing = [sid for sid, _, _ in SHOTS if sid not in dur]
    if missing:
        sys.exit(f"manifest missing segments: {missing}")

    # ---- timeline ----------------------------------------------------------
    V, A = [], []           # shot durations, global start of full visibility
    for sid, _, _ in SHOTS:
        V.append(round(LEAD + dur[sid] + TAIL, 3))
    acc = 0.0
    for i in range(len(SHOTS)):
        A.append(acc)
        acc += V[i] - XFADE
    total = sum(V) - XFADE * (len(SHOTS) - 1)
    B = [round(A[i] + LEAD, 3) for i in range(len(SHOTS))]  # audio starts

    # ---- filter graph ------------------------------------------------------
    parts, tips = [], []
    for i, (sid, img, motion) in enumerate(SHOTS):
        frames = max(2, round(V[i] * FPS))
        zp = zoompan_expr(motion, frames)
        parts.append(
            f"[{i}:v]scale=3840:2160:force_original_aspect_ratio=decrease,"
            f"pad=3840:2160:(ow-iw)/2:(oh-ih)/2:color={PAPER},setsar=1,"
            f"zoompan={zp}:d={frames}:s=1920x1080:fps={FPS},settb=AVTB[v{i}]"
        )
    # xfade chain
    prev = "v0"
    offset = V[0]
    for i in range(1, len(SHOTS)):
        lab = f"x{i}"
        parts.append(
            f"[{prev}][v{i}]xfade=transition=fade:duration={XFADE}:offset={round(offset - XFADE, 3)}[{lab}]"
        )
        prev = lab
        offset += V[i] - XFADE
    parts.append(
        f"[{prev}]fade=t=in:st=0:d=0.5,fade=t=out:st={round(total - 0.7, 3)}:d=0.7,"
        f"subtitles=subs.ass,format=yuv420p[vout]"
    )
    # audio: delay each wav to its slot, mix, normalise
    for i, (sid, _, _) in enumerate(SHOTS):
        ms = max(0, round(B[i] * 1000))
        parts.append(f"[{len(SHOTS) + i}:a]adelay={ms}|{ms}[a{i}]")
    amix_in = "".join(f"[a{i}]" for i in range(len(SHOTS)))
    parts.append(
        f"{amix_in}amix=inputs={len(SHOTS)}:duration=longest:normalize=0,"
        f"loudnorm=I=-16:TP=-1.5:LRA=11[aout]"
    )

    build_dir.mkdir(parents=True, exist_ok=True)
    (build_dir / "filter.txt").write_text(";\n".join(parts), encoding="utf-8")

    # ---- subtitles ---------------------------------------------------------
    events = []
    for i, (sid, _, _) in enumerate(SHOTS):
        start = B[i] + 0.10
        end = min(B[i] + dur[sid] + 0.30, (A[i + 1] if i + 1 < len(SHOTS) else total) - 0.05)
        events.append({"id": sid, "start": start, "end": max(end, start + 1.0), "text": text[sid]})
    build_ass(events, out_dir / "subs.ass")

    # ---- ffmpeg invocation (relative paths; Windows exe can't read /mnt/*) --
    cmd = [find_ffmpeg(), "-y"]
    for _, img, _ in SHOTS:
        cmd += ["-i", os.path.relpath(REPO / img, out_dir)]
    for sid, _, _ in SHOTS:
        cmd += ["-i", os.path.relpath(tts_dir / audio_file[sid], out_dir)]
    cmd += [
        "-filter_complex_script", os.path.relpath(build_dir / "filter.txt", out_dir),
        "-map", "[vout]", "-map", "[aout]",
        "-c:v", "libx264", "-preset", "slow", "-crf", "18",
        "-c:a", "aac", "-b:a", "192k",
        "-r", str(FPS), "-t", f"{total:.3f}", "-movflags", "+faststart",
        "final.mp4",
    ]
    print(f"shots: {[round(v,1) for v in V]}")
    print(f"total: {total:.1f}s  narration: {sum(dur.values()):.1f}s")
    print(" ".join(cmd[:8]), "...")
    if args.dry_run:
        return
    ret = subprocess.run(cmd, cwd=out_dir).returncode
    sys.exit(ret if ret else print(f"OK -> {out_dir / 'final.mp4'}"))


if __name__ == "__main__":
    main()
