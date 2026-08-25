#!/usr/bin/env python3
"""Build the 1-minute competition video v2: real-operation shots + cursor animation.

Reads artifacts/competition-video-v2/tts/manifest.json (from minute-video-tts-v2.json),
uses REAL app screenshots taken via opencli on the live instance, overlays an
animated cursor that glides toward the real click coordinates at the end of each
operation shot (screencast editing language), and burns KaiTi subtitles.

Usage:
    python3 scripts/build_minute_video2.py [--dry-run]
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

FPS = 30
XFADE = 0.45
LEAD = 0.45
TAIL = 0.6

PAPER = "0xfaf7f2"
SHOT = "artifacts/demo-shots"
PPT = "docs/experience/competition-ppt/shots"

# CSS-viewport click coords recorded while driving the real app (2560x~1216 viewport,
# screenshots are 3840x1823 physical = CSS x1.5, then padded to 3840x2160).
# (segment_id, image, motion, cursor_css_target_or_None)
SHOTS = [
    ("n1-hook",     f"{PPT}/s2.png",                    "zoom-in", None),
    ("n2-what",     f"{PPT}/s4.png",                    "zoom-in", None),
    ("n3-board",    f"{SHOT}/r2-articles.png",          "still",   (437, 348)),  # click tag chip
    ("n4-explain",  f"{SHOT}/r3-match-detail.png",      "still",   (509, 96)),   # -> daily tab
    ("n5-daily",    f"{SHOT}/r5-daily.png",             "still",   (425, 96)),   # -> topic overview
    ("n6-lanes",    f"{SHOT}/r7-lanes.png",             "still",   (664, 96)),   # -> data enrichment
    ("n7-agent",    f"{SHOT}/r10-agent-report.png",     "zoom-in", None),
    ("n8-close",    f"{PPT}/s15.png",                   "still",   None),
]

# CSS(2560-wide viewport) -> 1080p video coordinates:
#   phys = css * 1.5 ; padded into 3840x2160 (y += (2160-1823)/2) ; final = /2
PAD_Y = (2160 - 1823) / 2


def css_to_final(cx: float, cy: float) -> tuple[float, float]:
    return cx * 1.5 / 2, (cy * 1.5 + PAD_Y) / 2


def ass_time(t: float) -> str:
    h = int(t // 3600); m = int(t % 3600 // 60); s = t % 60
    return f"{h}:{m:02d}:{s:05.2f}"


def zoompan_expr(motion: str, frames: int) -> str:
    D = frames
    cx, cy = "iw/2-(iw/zoom)/2", "ih/2-(ih/zoom)/2"
    if motion == "zoom-in":
        return f"z='1+0.05*on/{D}':x='{cx}':y='{cy}'"
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
        style = "Dark" if ev["id"] == "n8-close" else "Light"
        text = ev["text"].replace("\n", "\\N")
        lines.append(
            f"Dialogue: 0,{ass_time(ev['start'])},{ass_time(ev['end'])},{style},,0,0,0,,{text}\n"
        )
    path.write_text("".join(lines), encoding="utf-8")


def make_cursor_pngs(out_dir: Path) -> None:
    """Draw a white/black arrow cursor and a red click dot (1080p scale)."""
    from PIL import Image, ImageDraw

    size = 60
    im = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    arrow = [(4, 2), (4, 44), (15, 34), (22, 50), (30, 46), (23, 31), (36, 31)]
    d.polygon(arrow, fill=(250, 247, 242, 245), outline=(26, 26, 26, 255), width=3)
    im.save(out_dir / "cursor.png")

    dot = Image.new("RGBA", (36, 36), (0, 0, 0, 0))
    dd = ImageDraw.Draw(dot)
    dd.ellipse([4, 4, 31, 31], outline=(217, 74, 74, 235), width=5)
    dot.save(out_dir / "clickdot.png")


def find_ffmpeg() -> str:
    win = Path("/mnt/d/tool/ffmpeg/ffmpeg.exe")
    if win.exists():
        return str(win)
    found = shutil.which("ffmpeg")
    if found:
        return found
    sys.exit("ffmpeg not found")


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--tts-dir", default="artifacts/competition-video-v2/tts")
    ap.add_argument("--out", default="artifacts/competition-video-v2")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    tts_dir = (REPO / args.tts_dir).resolve()
    out_dir = (REPO / args.out).resolve()
    build_dir = out_dir / "build"
    build_dir.mkdir(parents=True, exist_ok=True)

    manifest = json.loads((tts_dir / "manifest.json").read_text(encoding="utf-8"))
    seg = {s["id"]: s for s in manifest["segments"]}
    missing = [sid for sid, _, _, _ in SHOTS if sid not in seg]
    if missing:
        sys.exit(f"manifest missing segments: {missing} (run mimo_tts on minute-video-tts-v2.json first)")

    make_cursor_pngs(out_dir)

    # ---- timeline ----------------------------------------------------------
    V = [round(LEAD + seg[sid]["duration_seconds"] + TAIL, 3) for sid, _, _, _ in SHOTS]
    A, acc = [], 0.0
    for i in range(len(SHOTS)):
        A.append(acc)
        acc += V[i] - XFADE
    total = sum(V) - XFADE * (len(SHOTS) - 1)
    B = [round(A[i] + LEAD, 3) for i in range(len(SHOTS))]

    # ---- filter graph ------------------------------------------------------
    parts = []
    n = len(SHOTS)
    for i, (sid, img, motion, _) in enumerate(SHOTS):
        frames = max(2, round(V[i] * FPS))
        zp = zoompan_expr(motion, frames)
        parts.append(
            f"[{i}:v]scale=3840:2160:force_original_aspect_ratio=decrease,"
            f"pad=3840:2160:(ow-iw)/2:(oh-ih)/2:color={PAPER},setsar=1,"
            f"zoompan={zp}:d={frames}:s=1920x1080:fps={FPS},settb=AVTB[v{i}]"
        )
    prev = "v0"
    offset = V[0]
    for i in range(1, n):
        lab = f"x{i}"
        parts.append(
            f"[{prev}][v{i}]xfade=transition=fade:duration={XFADE}:offset={round(offset - XFADE, 3)}[{lab}]"
        )
        prev = lab
        offset += V[i] - XFADE

    # cursor overlays on the composed chain: glide in during the last 1.8s of each op shot
    CURSOR_IN = 1.8
    for k, (sid, _, _, target) in enumerate(SHOTS):
        if not target:
            continue
        tx, ty = css_to_final(*target)
        vis_start = A[k]
        vis_end = A[k] + V[k] - XFADE  # start fading into next
        glide_start = vis_end - CURSOR_IN - 0.15
        sx, sy = 1830, 820
        expr_x = f"'{sx:.0f}+({tx:.0f}-{sx:.0f})*min(1\\,max(0\\,(t-{glide_start:.2f})/1.1))'"
        expr_y = f"'{sy:.0f}+({ty:.0f}-{sy:.0f})*min(1\\,max(0\\,(t-{glide_start:.2f})/1.1))'"
        nxt = f"cur{k}"
        parts.append(
            f"[{prev}][{n}:v]overlay=x={expr_x}:y={expr_y}"
            f":enable='between(t,{glide_start:.2f},{vis_end:.2f})'[{nxt}]"
        )
        prev = nxt
        # click dot flashes right at the cut
        dot_t = vis_end - 0.35
        nxt2 = f"dot{k}"
        parts.append(
            f"[{prev}][{n + 1}:v]overlay=x={tx - 18:.0f}:y={ty - 18:.0f}"
            f":enable='between(t,{dot_t:.2f},{vis_end:.2f})'[{nxt2}]"
        )
        prev = nxt2

    parts.append(
        f"[{prev}]fade=t=in:st=0:d=0.5,fade=t=out:st={round(total - 0.7, 3)}:d=0.7,"
        f"subtitles=subs.ass,format=yuv420p[vout]"
    )
    for i, (sid, _, _, _) in enumerate(SHOTS):
        ms = max(0, round(B[i] * 1000))
        parts.append(f"[{n + 2 + i}:a]adelay={ms}|{ms}[a{i}]")
    amix_in = "".join(f"[a{i}]" for i in range(n))
    parts.append(
        f"{amix_in}amix=inputs={n}:duration=longest:normalize=0,"
        f"loudnorm=I=-16:TP=-1.5:LRA=11[aout]"
    )

    (build_dir / "filter.txt").write_text(";\n".join(parts), encoding="utf-8")

    # ---- subtitles ---------------------------------------------------------
    events = []
    for i, (sid, _, _, _) in enumerate(SHOTS):
        start = B[i] + 0.10
        end = min(B[i] + seg[sid]["duration_seconds"] + 0.30,
                  (A[i + 1] if i + 1 < n else total) - 0.05)
        events.append({"id": sid, "start": start, "end": max(end, start + 1.0),
                       "text": seg[sid]["text"]})
    build_ass(events, out_dir / "subs.ass")

    # ---- ffmpeg ------------------------------------------------------------
    cmd = [find_ffmpeg(), "-y"]
    for _, img, _, _ in SHOTS:
        cmd += ["-i", os.path.relpath(REPO / img, out_dir)]
    cmd += ["-i", "cursor.png", "-i", "clickdot.png"]
    for sid, _, _, _ in SHOTS:
        cmd += ["-i", os.path.relpath(tts_dir / seg[sid]["file"], out_dir)]
    cmd += [
        "-filter_complex_script", os.path.relpath(build_dir / "filter.txt", out_dir),
        "-map", "[vout]", "-map", "[aout]",
        "-c:v", "libx264", "-preset", "slow", "-crf", "18",
        "-c:a", "aac", "-b:a", "192k",
        "-r", str(FPS), "-t", f"{total:.3f}", "-movflags", "+faststart",
        "final.mp4",
    ]
    print(f"shots: {[round(v,1) for v in V]}")
    print(f"total: {total:.1f}s  narration: {sum(s['duration_seconds'] for s in seg.values()):.1f}s")
    if args.dry_run:
        print(" ".join(cmd[:10]), "...")
        return
    ret = subprocess.run(cmd, cwd=out_dir).returncode
    sys.exit(ret if ret else print(f"OK -> {out_dir / 'final.mp4'}"))


if __name__ == "__main__":
    main()
