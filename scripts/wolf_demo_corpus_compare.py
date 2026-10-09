#!/usr/bin/env python3
"""Compare all four built-in demos and optional original-format recordings."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
MEMORY_PROFILES = {
    "strict-source": None,
    "registered-apogee-v1.4-2a969a97": "2a969a97bc644d04be030d0d08bfda5477179d60f5dbfcc04616a18fa083ce77",
}


def memory_profile_metadata(name):
    if name not in MEMORY_PROFILES:
        raise ValueError(f"unknown original memory profile: {name}")
    return {"name": name, "executable_sha256": MEMORY_PROFILES[name]}


def arguments():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, help="selected WL6/WL1 data; default: embedded shareware")
    parser.add_argument("--source", type=Path, help="existing pinned id-Software/wolf3d checkout")
    parser.add_argument("--out", type=Path, default=ROOT / "build/wolf-demo-corpus-compare")
    parser.add_argument("--manifest", type=Path, help="JSON external recordings with paths and provenance")
    parser.add_argument("--sound-mode", choices=("off", "adlib", "adlib-digi"), default="adlib-digi",
                        help="original sound routing (default: adlib-digi)")
    parser.add_argument("--memory-profile", choices=tuple(MEMORY_PROFILES), default="strict-source",
                        help="strict source bounds or an explicitly identified DOS executable layout")
    parser.add_argument("--recorded-only", action="store_true", help="omit the four alternate-fire stress runs")
    parser.add_argument("--external-only", action="store_true", help="omit built-ins; requires a nonempty manifest")
    return parser.parse_args()


def external_cases(path, default_data):
    """Resolve and validate the complete corpus before running any comparison."""
    if not path:
        return []
    path = path.resolve()
    content = json.loads(path.read_text())
    if not isinstance(content, dict) or content.get("version") != 1 or not isinstance(content.get("recordings"), list):
        raise ValueError("manifest requires version 1 and a recordings array")
    cases = []
    for record in content["recordings"]:
        if not isinstance(record, dict):
            raise ValueError("each recording must be an object")
        for field in ("id", "file", "source_url", "recorded_version", "sha256"):
            if not isinstance(record.get(field), str) or not record[field]:
                raise ValueError(f"recording requires a nonempty {field}")
        if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]*", record["id"]):
            raise ValueError(f"unsafe recording id: {record['id']!r}")
        demo = (path.parent / record["file"]).resolve()
        raw = demo.read_bytes()
        if hashlib.sha256(raw).hexdigest() != record["sha256"].lower():
            raise ValueError(f"recording checksum mismatch: {demo}")
        if "data" in record and not isinstance(record["data"], str):
            raise ValueError("recording data must be a directory path string")
        data = (path.parent / record["data"]).resolve() if record.get("data") else default_data
        if data is not None and not data.is_dir():
            raise ValueError(f"missing data directory: {data}")
        expected_data = record.get("data_sha256", {})
        if not isinstance(expected_data, dict):
            raise ValueError("data_sha256 must be a filename/checksum object")
        if expected_data and data is None:
            data = ROOT / "internal/wl6/shareware"
        for name, digest in expected_data.items():
            if not isinstance(name, str) or Path(name).name != name or not isinstance(digest, str):
                raise ValueError(f"invalid data fingerprint: {name!r}")
            if hashlib.sha256((data / name).read_bytes()).hexdigest() != digest.lower():
                raise ValueError(f"data checksum mismatch: {data / name}")
        cases.append({"id": "external-" + record["id"], "data": data, "demo_file": demo,
                      "extra_fire": False, "provenance": record})
    return cases


def main():
    args = arguments()
    data = args.data.resolve() if args.data else None
    if data is not None and not data.is_dir():
        raise ValueError(f"missing data directory: {data}")
    cases = []
    if not args.external_only:
        for index in range(4):
            for extra in ([False] if args.recorded_only else [False, True]):
                cases.append({"id": f"demo-{index}" + ("-extra-fire" if extra else ""),
                              "data": data, "demo_index": index, "extra_fire": extra})
    external = external_cases(args.manifest, data)
    if args.external_only and not external:
        raise ValueError("--external-only requires a nonempty --manifest")
    cases.extend(external)
    ids = [case["id"] for case in cases]
    if len(set(ids)) != len(ids):
        raise ValueError("duplicate recording ids")
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    summary = {"version": 1, "status": "running", "memory_profile": memory_profile_metadata(args.memory_profile), "cases": []}
    summary_path = out / "summary.json"
    summary_path.write_text(json.dumps(summary, indent=2) + "\n")
    for case in cases:
        run_out = out / case["id"]
        run_out.mkdir(parents=True, exist_ok=True)
        # A compiler or preflight failure must not inherit a prior run's result.
        result_path = run_out / "result.json"
        result_path.unlink(missing_ok=True)
        (run_out / "raycast/result.json").unlink(missing_ok=True)
        command = [str(ROOT / "scripts/wolf_demo_runtime_compare.sh"), "--out", str(run_out)]
        command.extend(["--sound-mode", args.sound_mode])
        command.extend(["--memory-profile", args.memory_profile])
        if case["data"] is not None:
            command.extend(["--data", str(case["data"])])
        if args.source:
            command.extend(["--source", str(args.source.resolve())])
        if "demo_file" in case:
            command.extend(["--demo-file", str(case["demo_file"])])
        else:
            command.extend(["--demo-index", str(case["demo_index"])])
        if case["extra_fire"]:
            command.append("--extra-fire")
        print(f"Running {case['id']} ...", flush=True)
        with (run_out / "run.log").open("w") as log:
            completed = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, cwd=ROOT)
        result = {"id": case["id"], "exit_code": completed.returncode, "output": str(run_out),
                  "command": command, "provenance": case.get("provenance")}
        if result_path.exists():
            try:
                result["runtime"] = json.loads(result_path.read_text())
            except (OSError, ValueError) as error:
                result["result_error"] = str(error)
        result["status"] = "passed" if completed.returncode == 0 else "failed"
        if "runtime" not in result or "result_error" in result:
            result["status"] = "failed"
        summary["cases"].append(result)
        summary_path.write_text(json.dumps(summary, indent=2) + "\n")
        print(f"  {result['status']}: {run_out / 'run.log'}", flush=True)
    summary["status"] = "passed" if all(c["status"] == "passed" for c in summary["cases"]) else "failed"
    summary_path.write_text(json.dumps(summary, indent=2) + "\n")
    print(f"Corpus {summary['status']}: {summary_path}")
    return 0 if summary["status"] == "passed" else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, TypeError) as error:
        print(f"wolf_demo_corpus_compare: {error}", file=sys.stderr)
        sys.exit(2)
