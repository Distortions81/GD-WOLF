#!/usr/bin/env python3
"""Compare an external demo corpus with one frozen C/Go build and per-route traces."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

from wolf_demo_corpus_compare import MEMORY_PROFILES, external_cases, memory_profile_metadata


ROOT = Path(__file__).resolve().parents[1]


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2) + "\n")
    temporary.replace(path)


def read_result(path):
    try:
        value = json.loads(path.read_text())
        if not isinstance(value, dict):
            raise ValueError("result must be an object")
        return value, None
    except (OSError, ValueError) as error:
        return None, str(error)


def occupancy_environment(env, enabled):
    result = env.copy()
    # Explicitly remove inherited values when this audit is disabled.
    result.pop("GDWOLF_DEMO_OCCUPANCY", None)
    if enabled:
        result["GDWOLF_DEMO_OCCUPANCY"] = "1"
    return result


def source_fingerprints():
    paths = list(ROOT.glob("*.go")) + list((ROOT / "internal").rglob("*.go"))
    paths += list((ROOT / "tools/wolfsrc-reference").glob("*.py"))
    paths += list((ROOT / "tools/wolfsrc-reference").glob("*.c"))
    paths += list((ROOT / "tools/wolfsrc-reference").glob("*.inc"))
    paths += [ROOT / "go.mod", ROOT / "go.sum"]
    return {str(p.relative_to(ROOT)): hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(paths)}


def run(command, log, env, timeout=300):
    with log.open("w") as output:
        try:
            completed = subprocess.run(command, cwd=ROOT, env=env, stdout=output,
                                       stderr=subprocess.STDOUT, timeout=timeout)
            return completed.returncode
        except subprocess.TimeoutExpired:
            output.write(f"\nSweep timeout after {timeout} seconds.\n")
            return 124


def matched_coverage(path, matched):
    """Report observed matched samples, not claims about unexplored map regions."""
    tiles, angles, kinds, active_kinds, actor_states, door_actions, weapons, sounds = (set() for _ in range(8))
    samples = damage = shots = 0
    if not path.exists():
        return {"samples": 0}
    with path.open() as trace:
        for line in trace:
            sample = json.loads(line)
            if sample["command"] < 0 or sample["command"] >= matched:
                continue
            samples += 1
            state = sample["state"]
            player = state["player"]
            tiles.add((player["x"] // 65536, player["y"] // 65536))
            angles.add(player["angle"])
            for actor in state["actors"]:
                kinds.add(actor["kind"])
                actor_states.add((actor["kind"], actor["state"]))
                if actor["active"]:
                    active_kinds.add(actor["kind"])
            door_actions.update(door["action"] for door in state["doors"])
            weapons.add(state["weapon"]["type"])
            shots += state["weapon"]["shots"]
            damage += state["damage"]
            sounds.add(state["sound"]["playing"])
    return {"source_trace": path.name, "samples": samples, "player_tiles": sorted(tiles), "player_angles": sorted(angles),
            "actor_kinds": sorted(kinds), "active_actor_kinds": sorted(active_kinds),
            "actor_states": sorted(actor_states), "door_actions": sorted(door_actions),
            "weapons": sorted(weapons), "synthesized_sounds": sorted(sounds),
            "shots": shots, "player_damage": damage}


def arguments():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True, help="new output directory; never reuses stale results")
    parser.add_argument("--source", type=Path, default=ROOT / "build/wolfsrc-source")
    parser.add_argument("--sound-mode", choices=("off", "adlib", "adlib-digi"), default="adlib-digi")
    parser.add_argument("--memory-profile", choices=tuple(MEMORY_PROFILES), default="strict-source",
                        help="strict source bounds or an explicitly identified DOS executable layout")
    parser.add_argument("--maps", nargs="+", type=int, help="run selected map indices from the manifest")
    parser.add_argument("--patterns", nargs="+", help="run selected generated patterns from the manifest")
    parser.add_argument("--actor-occupancy", action="store_true", help="also compare the original actor occupancy grid and pool slots")
    parser.add_argument("--runtime-only", action="store_true", help="explicitly omit x86 floor-mask audit")
    return parser.parse_args()


def main():
    args = arguments()
    cases = external_cases(args.manifest, None)
    if args.maps is not None:
        cases = [case for case in cases if case["provenance"].get("map") in args.maps]
    if args.patterns is not None:
        cases = [case for case in cases if case["provenance"].get("pattern") in args.patterns]
    if not cases or len({case["id"] for case in cases}) != len(cases):
        raise ValueError("selected corpus is empty or has duplicate IDs")
    out, source = args.out.resolve(), args.source.resolve()
    if out.exists():
        raise ValueError(f"output directory already exists: {out}")
    # One X server serves the whole batch; all game executions remain separate.
    if sys.platform.startswith("linux") and not os.environ.get("DISPLAY"):
        return subprocess.call(["xvfb-run", "-a", sys.executable, str(Path(__file__).resolve()), *sys.argv[1:]])
    out.mkdir(parents=True)
    summary = {"version": 1, "status": "building", "manifest": str(args.manifest.resolve()),
               "sound_mode": args.sound_mode, "visibility_audit": not args.runtime_only,
               "memory_profile": memory_profile_metadata(args.memory_profile),
               "actor_occupancy_audit": args.actor_occupancy or os.environ.get("GDWOLF_DEMO_OCCUPANCY") == "1",
               "area_plane_audit": os.environ.get("GDWOLF_DEMO_AREA_PLANE") == "1",
               "statics_audit": os.environ.get("GDWOLF_DEMO_STATICS") == "1",
               "planned_cases": len(cases), "cases": []}
    summary_path = out / "summary.json"
    write_json(summary_path, summary)
    before = source_fingerprints()
    write_json(out / "source-fingerprints.json", before)
    env = os.environ.copy()
    env.update({"GOMAXPROCS": "1", "GOMEMLIMIT": os.environ.get("GDWOLF_GO_MEM_LIMIT", "12GiB"),
                "GOCACHE": os.environ.get("GOCACHE", "/tmp/gd-wolf-go-cache"), "PYTHONDONTWRITEBYTECODE": "1"})
    env = occupancy_environment(env, summary["actor_occupancy_audit"])
    env["GDWOLF_DEMO_MEMORY_PROFILE"] = args.memory_profile
    reference = out / "wolf-demo-runtime-reference"
    binary = out / "wolf-demo-tests"
    build_commands = [
        [sys.executable, str(ROOT / "tools/wolfsrc-reference/build_demo_runtime.py"), "--source", str(source), "--output", str(reference)],
        ["go", "test", "-c", "-o", str(binary), "."],
    ]
    for index, command in enumerate(build_commands):
        code = run(command, out / f"build-{index}.log", env)
        if code:
            summary.update(status="build_failed", build_exit_code=code, build_log=str(out / f"build-{index}.log"))
            write_json(summary_path, summary)
            return 1
    if before != source_fingerprints():
        summary.update(status="source_changed_during_build")
        write_json(summary_path, summary)
        print("Source changed while compiling; rerun in a new output directory.", file=sys.stderr)
        return 1
    write_json(out / "binary-fingerprints.json", {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in (reference, binary)})
    env.update({"GDWOLF_DEMO_RUNTIME_REFERENCE": str(reference), "GDWOLF_DEMO_SOUND_MODE": args.sound_mode,
                "GDWOLF_DEMO_RUNTIME_DATA": str(cases[0]["data"] or ""),
                "GDWOLF_DEMO_EXTRA_FIRE": "0", "GDWOLF_DEMO_INDEX": "0"})
    # Static boundary probes run once per immutable binary, not once per route.
    summary["probe_command"] = [str(binary), "-test.run", "^TestWolfDemo(Projection|Rewards|Elevator|Sound|Victory|VictoryEntry)Compare$",
                                "-test.count=1", "-test.parallel=1", "-test.v"]
    # Grid assertions stay enabled for every route. The independent boundary
    # probes have their own state assertions and do not need a 4096-cell dump
    # for each of the thousands of victory configurations.
    summary["probe_occupancy"] = False
    summary["probe_area_plane"] = False
    summary["probe_statics"] = False
    probe_env = occupancy_environment(env, False)
    probe_env.pop("GDWOLF_DEMO_AREA_PLANE", None)
    probe_env.pop("GDWOLF_DEMO_STATICS", None)
    summary["status"] = "probing"
    write_json(summary_path, summary)
    summary["probe_exit_code"] = run(summary["probe_command"], out / "probes.log", probe_env)
    summary["status"] = "running"
    write_json(summary_path, summary)
    for index, case in enumerate(cases):
        run_out = out / case["id"]
        run_out.mkdir()
        shutil.copy2(reference, run_out / reference.name)
        write_json(run_out / "input-manifest.json", {"provenance": case["provenance"], "sound_mode": args.sound_mode,
                                                    "memory_profile": memory_profile_metadata(args.memory_profile),
                                                    "actor_occupancy_audit": summary["actor_occupancy_audit"],
                                                    "area_plane_audit": summary["area_plane_audit"],
                                                    "statics_audit": summary["statics_audit"],
                                                    "reference_revision": "05167784ef009d0d0daefe8d012b027f39dc8541"})
        case_env = env | {"GDWOLF_DEMO_FILE": str(case["demo_file"]), "GDWOLF_DEMO_RUNTIME_DATA": str(case["data"] or ""),
                          "GDWOLF_DEMO_RUNTIME_OUT": str(run_out), "GDWOLF_DEMO_RAYCAST_OUT": str(run_out / "raycast-input.jsonl")}
        command = [str(binary), "-test.run", "^TestWolfDemoRuntimeCompare$", "-test.count=1", "-test.timeout=4m", "-test.v"]
        code = run(command, run_out / "compare.log", case_env)
        runtime, error = read_result(run_out / "result.json")
        result = {"id": case["id"], "status": "failed", "exit_code": code, "output": str(run_out),
                  "provenance": case["provenance"], "runtime": runtime}
        if error:
            result["result_error"] = error
        if runtime:
            try:
                result["matched_coverage"] = matched_coverage(run_out / "reference.jsonl", runtime.get("matched_commands", 0))
            except (OSError, ValueError, KeyError, TypeError) as error:
                result["coverage_error"] = str(error)
        if code == 0 and runtime and runtime.get("status") in ("success", "matched_terminal_state"):
            if args.runtime_only:
                result["status"] = "runtime_passed"
            else:
                audit_code = run([str(ROOT / "scripts/wolf_demo_raycast_compare.sh"), "--source", str(source),
                                  "--runtime", str(run_out), "--out", str(run_out / "raycast")], run_out / "raycast.log", env)
                result["raycast_exit_code"] = audit_code
                result["raycast"], audit_error = read_result(run_out / "raycast/result.json")
                if audit_error:
                    result["raycast_error"] = audit_error
                if (audit_code == 0 and result["raycast"] and result["raycast"].get("status") == "success"
                        and result["raycast"].get("matched_commands") == runtime.get("matched_commands")):
                    result["status"] = "passed"
        if result["status"] == "failed":
            lines = (run_out / "compare.log").read_text(errors="replace").splitlines()
            details = [line[:1200] for line in lines
                       if "unsupported" in line.lower() or ".go:" in line or "FAIL" in line]
            result["failure_detail"] = details if len(details) <= 12 else details[:6] + details[-6:]
            result["failure_kind"] = ("unsupported_original_memory_access"
                                      if any("unsupported original memory access:" in line for line in lines)
                                      else "comparison_failure")
        summary["cases"].append(result)
        write_json(summary_path, summary)
        print(f"{index + 1}/{len(cases)} {case['id']}: {result['status']} ({runtime.get('matched_commands', 0) if runtime else 0} commands)", flush=True)
    expected = "runtime_passed" if args.runtime_only else "passed"
    summary["status"] = expected if summary["probe_exit_code"] == 0 and all(c["status"] == expected for c in summary["cases"]) else "failed"
    summary["source_changed_after_build"] = before != source_fingerprints()
    write_json(summary_path, summary)
    print(f"Sweep {summary['status']}: {summary_path}")
    return 0 if summary["status"] == expected else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        print(f"wolf_demo_sweep_compare: {error}", file=sys.stderr)
        sys.exit(2)
