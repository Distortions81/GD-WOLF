#!/usr/bin/env python3
"""Generate reproducible native-input demo routes without changing game state."""

import argparse
import hashlib
import json
from pathlib import Path
import struct
import sys


VERSION = 1
MAX_COMMANDS = (65535 - 4) // 3
PATTERNS = ("idle", "patrol", "combat")
DATA_NAMES = ("MAPHEAD", "GAMEMAPS", "VSWAP", "VGADICT", "VGAHEAD", "VGAGRAPH", "AUDIOHED", "AUDIOT")
ATTACK, STRAFE, RUN, USE = 1, 2, 4, 8


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def map_selection(value):
    maps = []
    for item in value.split(","):
        parts = item.split("-")
        try:
            if len(parts) == 1:
                selected = [int(parts[0])]
            elif len(parts) == 2:
                first, last = map(int, parts)
                if first > last or first < 0 or last >= 60:
                    raise ValueError()
                selected = list(range(first, last + 1))
            else:
                raise ValueError()
        except ValueError:
            raise ValueError(f"invalid map selection: {value!r}") from None
        if any(m < 0 or m >= 60 for m in selected):
            raise ValueError("map indices must be within 0..59")
        maps.extend(selected)
    if len(set(maps)) != len(maps):
        raise ValueError("map selection contains duplicate indices")
    return maps


class RouteRandom:
    """Explicit uint32 LCG: independent of Python random implementation changes."""

    def __init__(self, seed):
        self.state = seed & 0xffffffff

    def next(self):
        self.state = (1664525 * self.state + 1013904223) & 0xffffffff
        return self.state


def route_commands(pattern, count, seed):
    if pattern not in PATTERNS or not 1 <= count <= MAX_COMMANDS:
        raise ValueError("invalid pattern or native-format command count")
    if pattern == "idle":
        return [(0, 0, 0)] * count
    rng = RouteRandom(seed)
    commands = [(0, 0, 0)] * min(8, count)
    # Long straight sections can reach doors; shorter turns and strafes change
    # the path. Neither map contents nor observed port state guide these inputs.
    motions = ((0, -100, 0), (0, -70, RUN), (25, 0, 0), (-25, 0, 0),
               (70, -50, STRAFE), (-70, -50, STRAFE), (0, 70, 0), (7, -85, 0))
    block = 0
    while len(commands) < count:
        choice = rng.next()
        motion = 0 if block % 3 == 0 else (choice >> 16) % len(motions)
        x, y, base_buttons = motions[motion]
        length = 32 + (rng.next() >> 24) % 33 if motion < 2 else 8 + (rng.next() >> 24) % 17
        for step in range(length):
            buttons = base_buttons
            # Fresh and held use presses exercise the original button latch.
            if step % 12 in (0, 1):
                buttons |= USE
            if pattern == "combat":
                if step % 16 in range(4, 15):
                    buttons |= ATTACK
                if step == 3:
                    buttons |= 1 << (4 + ((choice >> 28) & 3))
            commands.append((buttons, x, y))
            if len(commands) == count:
                break
        block += 1
    return commands


def native_demo(map_index, commands):
    if not 0 <= map_index < 60 or not 1 <= len(commands) <= MAX_COMMANDS:
        raise ValueError("demo map or length is outside the native format")
    return struct.pack("<BHB", map_index, 4 + 3 * len(commands), 0) + b"".join(
        struct.pack("<Bbb", *command) for command in commands)


def generate(data, out, maps, patterns, seeds, count):
    if out.exists():
        raise ValueError(f"output directory already exists: {out}")
    if not maps or not patterns or not seeds or len(set(maps)) != len(maps) or len(set(patterns)) != len(patterns) or len(set(seeds)) != len(seeds):
        raise ValueError("maps, patterns and seeds must be nonempty and unique")
    if not 1 <= count <= MAX_COMMANDS or any(not 0 <= seed <= 0xffffffff for seed in seeds):
        raise ValueError("command count or uint32 seed is out of range")
    data = data.resolve()
    assets = {p.name: digest(p.read_bytes()) for p in sorted(data.iterdir())
              if p.is_file() and p.stem.upper() in DATA_NAMES and p.suffix.upper() in (".WL1", ".WL6")}
    if len(assets) != len(DATA_NAMES):
        raise ValueError("data directory must contain exactly one complete WL1 or WL6 asset set")
    extension = Path(next(iter(assets))).suffix.lower()
    if {Path(name).stem.upper() for name in assets} != set(DATA_NAMES) or len({Path(name).suffix.lower() for name in assets}) != 1:
        raise ValueError("data directory contains mixed or incomplete asset variants")
    if extension == ".wl1" and max(maps) >= 10:
        raise ValueError("shareware data has only map slots 0..9")
    prepared = []
    script = Path(__file__).resolve()
    for map_index in maps:
        for pattern in patterns:
            for seed in seeds:
                route_seed = (seed ^ (map_index * 0x9e3779b9)) & 0xffffffff
                commands = route_commands(pattern, count, route_seed)
                raw = native_demo(map_index, commands)
                name = f"map-{map_index:02d}-{pattern}-seed-{seed:08x}"
                record = {
                    "id": name, "file": f"recordings/{name}{extension}", "data": str(data),
                    "source_url": script.as_uri(),
                    "recorded_version": f"Generated input corpus v{VERSION}; not a human recording",
                    "sha256": digest(raw), "data_sha256": assets,
                    "map": map_index, "commands": count, "pattern": pattern,
                    "seed": seed, "route_seed": route_seed,
                }
                prepared.append((record, raw))
    (out / "recordings").mkdir(parents=True)
    for record, raw in prepared:
        (out / record["file"]).write_bytes(raw)
    (out / "generator.py").write_bytes(script.read_bytes())
    manifest = {
        "version": 1,
        "generator": {"version": VERSION, "source": "generator.py", "script_sha256": digest(script.read_bytes()),
                      "maps": maps, "patterns": patterns, "seeds": seeds, "commands": count,
                      "scope": "Fixed input only; ordinary game spawn, difficulty, inventory and RNG; no state overrides"},
        "recordings": [record for record, _ in prepared],
    }
    (out / "corpus.json").write_text(json.dumps(manifest, indent=2) + "\n")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data", type=Path, required=True, help="local registered WL6 or shareware WL1 assets")
    parser.add_argument("--out", type=Path, required=True, help="new corpus directory, preferably under build/")
    parser.add_argument("--maps", default="0-59", help="comma-separated indices/ranges (default: all 60 registered maps)")
    parser.add_argument("--patterns", nargs="+", choices=PATTERNS, default=list(PATTERNS))
    parser.add_argument("--seeds", nargs="+", type=lambda s: int(s, 0), default=[0])
    parser.add_argument("--commands", type=int, default=512, help="commands per route; each advances four Wolf tics")
    args = parser.parse_args()
    out = args.out.resolve()
    result = generate(args.data, out, map_selection(args.maps), args.patterns, args.seeds, args.commands)
    print(f"Generated {len(result['recordings'])} fixed-input routes: {out / 'corpus.json'}")
    print("Compare with scripts/wolf_demo_corpus_compare.py --external-only --manifest " + str(out / "corpus.json"))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, struct.error) as error:
        print(f"wolf_generate_demo_corpus: {error}", file=sys.stderr)
        sys.exit(2)
