#!/usr/bin/env python3
"""Generate segmented MiMo V2.5 TTS audio, subtitles, and a merged narration."""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request
import wave
from dataclasses import dataclass
from pathlib import Path
from typing import Any


DEFAULT_BASE_URL = "https://api.xiaomimimo.com/v1"
DEFAULT_MODEL = "mimo-v2.5-tts"
DEFAULT_VOICE = "白桦"
DEFAULT_INSTRUCTION = (
    "使用自然、沉稳、有亲和力的中文普通话男声。"
    "语速中等，表达清晰，像独立开发者在介绍自己的产品。"
    "避免广告腔和过度煽情；重点句适度停顿，技术名词读得准确。"
)


@dataclass(frozen=True)
class Segment:
    segment_id: str
    text: str
    instruction: str
    voice: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate MiMo V2.5 TTS WAV files and SRT subtitles."
    )
    parser.add_argument(
        "input",
        type=Path,
        help="JSON script or UTF-8 text file. Text files are split by blank lines.",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=Path("artifacts/product-video/tts"),
        help="Output directory (default: artifacts/product-video/tts).",
    )
    parser.add_argument(
        "--model",
        default=os.environ.get("MIMO_TTS_MODEL", DEFAULT_MODEL),
        help=f"MiMo model ID (default: {DEFAULT_MODEL}).",
    )
    parser.add_argument(
        "--voice",
        default=os.environ.get("MIMO_TTS_VOICE", DEFAULT_VOICE),
        help=f"Preset voice ID (default: {DEFAULT_VOICE}).",
    )
    parser.add_argument(
        "--instruction",
        default=os.environ.get("MIMO_TTS_INSTRUCTION", DEFAULT_INSTRUCTION),
        help="Natural-language speaking-style instruction.",
    )
    parser.add_argument(
        "--base-url",
        default=os.environ.get("MIMO_BASE_URL", DEFAULT_BASE_URL),
        help=f"MiMo API base URL (default: {DEFAULT_BASE_URL}).",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=300,
        help="Per-request timeout in seconds (default: 300).",
    )
    parser.add_argument(
        "--retries",
        type=int,
        default=3,
        help="Retry count for rate limits and server errors (default: 3).",
    )
    parser.add_argument(
        "--overwrite",
        action="store_true",
        help="Regenerate WAV files that already exist.",
    )
    parser.add_argument(
        "--no-merge",
        action="store_true",
        help="Do not merge segment WAV files with ffmpeg.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate and print the plan without calling the API.",
    )
    return parser.parse_args()


def clean_segment_id(value: str, index: int) -> str:
    cleaned = re.sub(r"[^A-Za-z0-9_-]+", "-", value.strip()).strip("-")
    return cleaned or f"segment-{index:02d}"


def load_segments(
    input_path: Path, default_instruction: str, default_voice: str
) -> list[Segment]:
    if not input_path.exists():
        raise ValueError(f"Input file does not exist: {input_path}")

    if input_path.suffix.lower() == ".json":
        payload = json.loads(input_path.read_text(encoding="utf-8"))
        if isinstance(payload, list):
            raw_segments = payload
            defaults: dict[str, Any] = {}
        elif isinstance(payload, dict):
            raw_segments = payload.get("segments")
            defaults = payload.get("defaults") or {}
        else:
            raise ValueError("JSON input must be an array or an object with 'segments'.")

        if not isinstance(raw_segments, list):
            raise ValueError("JSON field 'segments' must be an array.")

        instruction = str(defaults.get("instruction") or default_instruction)
        voice = str(defaults.get("voice") or default_voice)
        segments: list[Segment] = []
        for index, item in enumerate(raw_segments, start=1):
            if not isinstance(item, dict):
                raise ValueError(f"Segment {index} must be an object.")
            text = str(item.get("text") or "").strip()
            if not text:
                raise ValueError(f"Segment {index} has no text.")
            segment_id = clean_segment_id(str(item.get("id") or ""), index)
            segments.append(
                Segment(
                    segment_id=segment_id,
                    text=text,
                    instruction=str(item.get("instruction") or instruction),
                    voice=str(item.get("voice") or voice),
                )
            )
        return segments

    paragraphs = [
        paragraph.strip()
        for paragraph in re.split(r"\r?\n\s*\r?\n", input_path.read_text("utf-8"))
        if paragraph.strip()
    ]
    return [
        Segment(
            segment_id=f"segment-{index:02d}",
            text=text,
            instruction=default_instruction,
            voice=default_voice,
        )
        for index, text in enumerate(paragraphs, start=1)
    ]


def request_audio(
    *,
    api_key: str,
    base_url: str,
    model: str,
    segment: Segment,
    timeout: int,
    retries: int,
) -> bytes:
    endpoint = base_url.rstrip("/") + "/chat/completions"
    payload = {
        "model": model,
        "messages": [
            {"role": "user", "content": segment.instruction},
            {"role": "assistant", "content": segment.text},
        ],
        "audio": {"format": "wav", "voice": segment.voice},
        "stream": False,
    }
    request = urllib.request.Request(
        endpoint,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={
            "api-key": api_key,
            "Content-Type": "application/json",
        },
        method="POST",
    )

    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                result = json.loads(response.read().decode("utf-8"))
            audio_data = result["choices"][0]["message"]["audio"]["data"]
            return base64.b64decode(audio_data)
        except urllib.error.HTTPError as exc:
            error_body = exc.read().decode("utf-8", errors="replace")
            retryable = exc.code == 429 or 500 <= exc.code < 600
            if not retryable or attempt >= retries:
                raise RuntimeError(
                    f"MiMo API returned HTTP {exc.code}: {error_body}"
                ) from exc
        except (urllib.error.URLError, TimeoutError) as exc:
            if attempt >= retries:
                raise RuntimeError(f"MiMo API request failed: {exc}") from exc
        except (KeyError, TypeError, ValueError) as exc:
            raise RuntimeError("MiMo response did not contain audio data.") from exc

        delay = min(2**attempt, 8)
        print(f"  retrying in {delay}s...", file=sys.stderr)
        time.sleep(delay)

    raise AssertionError("unreachable")


def wav_duration(path: Path) -> float:
    try:
        with wave.open(str(path), "rb") as audio:
            return audio.getnframes() / audio.getframerate()
    except (wave.Error, EOFError):
        ffprobe = shutil.which("ffprobe")
        if not ffprobe:
            raise RuntimeError(f"Cannot read WAV duration and ffprobe is unavailable: {path}")
        result = subprocess.run(
            [
                ffprobe,
                "-v",
                "error",
                "-show_entries",
                "format=duration",
                "-of",
                "default=noprint_wrappers=1:nokey=1",
                str(path),
            ],
            check=True,
            capture_output=True,
            text=True,
        )
        return float(result.stdout.strip())


def srt_timestamp(seconds: float) -> str:
    total_ms = max(0, round(seconds * 1000))
    hours, remainder = divmod(total_ms, 3_600_000)
    minutes, remainder = divmod(remainder, 60_000)
    secs, millis = divmod(remainder, 1000)
    return f"{hours:02d}:{minutes:02d}:{secs:02d},{millis:03d}"


def write_srt(segments: list[Segment], paths: list[Path], output: Path) -> list[float]:
    cursor = 0.0
    durations: list[float] = []
    lines: list[str] = []
    for index, (segment, path) in enumerate(zip(segments, paths, strict=True), start=1):
        duration = wav_duration(path)
        durations.append(duration)
        lines.extend(
            [
                str(index),
                f"{srt_timestamp(cursor)} --> {srt_timestamp(cursor + duration)}",
                segment.text,
                "",
            ]
        )
        cursor += duration
    output.write_text("\n".join(lines), encoding="utf-8-sig")
    return durations


def merge_wavs(paths: list[Path], output_dir: Path) -> Path | None:
    ffmpeg = shutil.which("ffmpeg")
    if not ffmpeg:
        print("ffmpeg not found; keeping individual WAV files.", file=sys.stderr)
        return None

    concat_file = output_dir / "concat.txt"
    concat_lines = []
    for path in paths:
        escaped_path = path.resolve().as_posix().replace("'", "'\\''")
        concat_lines.append(f"file '{escaped_path}'")
    concat_file.write_text("\n".join(concat_lines), encoding="utf-8")
    output = output_dir / "narration.wav"
    subprocess.run(
        [
            ffmpeg,
            "-y",
            "-hide_banner",
            "-loglevel",
            "error",
            "-f",
            "concat",
            "-safe",
            "0",
            "-i",
            str(concat_file),
            "-ar",
            "24000",
            "-ac",
            "1",
            "-c:a",
            "pcm_s16le",
            str(output),
        ],
        check=True,
    )
    return output


def main() -> int:
    args = parse_args()
    try:
        segments = load_segments(args.input, args.instruction, args.voice)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    print(f"Input: {args.input}")
    print(f"Segments: {len(segments)}")
    print(f"Characters: {sum(len(segment.text) for segment in segments)}")
    print(f"Model: {args.model}")
    print(f"Output: {args.output}")
    for index, segment in enumerate(segments, start=1):
        print(
            f"  {index:02d} {segment.segment_id}: "
            f"{len(segment.text)} chars, voice={segment.voice}"
        )

    if args.dry_run:
        return 0

    api_key = os.environ.get("MIMO_API_KEY", "").strip()
    if not api_key:
        print(
            "error: MIMO_API_KEY is not set. "
            "Set it in the current shell before running.",
            file=sys.stderr,
        )
        return 2

    args.output.mkdir(parents=True, exist_ok=True)
    audio_paths: list[Path] = []
    for index, segment in enumerate(segments, start=1):
        path = args.output / f"{index:02d}-{segment.segment_id}.wav"
        audio_paths.append(path)
        if path.exists() and not args.overwrite:
            print(f"[{index:02d}/{len(segments):02d}] reuse {path.name}")
            continue

        print(f"[{index:02d}/{len(segments):02d}] synthesize {segment.segment_id}")
        audio = request_audio(
            api_key=api_key,
            base_url=args.base_url,
            model=args.model,
            segment=segment,
            timeout=args.timeout,
            retries=args.retries,
        )
        path.write_bytes(audio)

    subtitle_path = args.output / "narration.srt"
    durations = write_srt(segments, audio_paths, subtitle_path)
    merged_path = None if args.no_merge else merge_wavs(audio_paths, args.output)

    manifest = {
        "source": str(args.input),
        "model": args.model,
        "segments": [
            {
                "id": segment.segment_id,
                "text": segment.text,
                "voice": segment.voice,
                "instruction": segment.instruction,
                "file": path.name,
                "duration_seconds": round(duration, 3),
            }
            for segment, path, duration in zip(
                segments, audio_paths, durations, strict=True
            )
        ],
        "subtitle": subtitle_path.name,
        "merged_audio": merged_path.name if merged_path else None,
        "total_duration_seconds": round(sum(durations), 3),
    }
    (args.output / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )

    print(f"Subtitles: {subtitle_path}")
    if merged_path:
        print(f"Merged audio: {merged_path}")
    print(f"Total duration: {sum(durations):.1f}s")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
