#!/usr/bin/env python3
"""Compile selected, verified original routines; never rewrite their decisions."""
import argparse
import hashlib
import pathlib
import re
import subprocess

SOURCE_REVISION = "05167784ef009d0d0daefe8d012b027f39dc8541"
SOURCE_HASHES = {
    "WL_STATE.C": "64f0f71a2e59f6c4077e6a03cebcdfff697ec8389acc665dfb75105c3a8fbbcc",
    "ID_US_A.ASM": "bc2d15e24be9cf9ad3bbd779b520560f9a52c3e4e51e2e6e122e6bfc169afa7c",
}


def build(source, output, cc):
    texts = {}
    for name, expected in SOURCE_HASHES.items():
        raw = (source / "WOLFSRC" / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise SystemExit(f"{name}: source differs from pinned revision {SOURCE_REVISION}")
        texts[name] = raw.decode("ascii")

    state = texts["WL_STATE.C"]
    tables = state[state.index("dirtype opposite[9]"):state.index("void\tSpawnNewObj")]
    movement = state[state.index("#define CHECKDIAG"):state.index("void MoveObj (objtype *ob, long move)\n{")]
    # DOS used 16-bit unsigned for tagged actor pointers. Keep the exact
    # comparisons and tags, but preserve host pointers on 32/64-bit machines.
    if movement.count("(unsigned)actorat") != 2 or movement.count("unsigned\ttemp;") != 1:
        raise SystemExit("unexpected pointer conversion sites in original TryWalk")
    movement = movement.replace("(unsigned)actorat", "(uintptr_t)actorat")
    movement = movement.replace("unsigned\ttemp;", "uintptr_t\ttemp;")

    asm = texts["ID_US_A.ASM"]
    table = asm[asm.index("rndtable db"):asm.index("PUBLIC\trndtable")]
    values = re.findall(r"\b\d+\b", table)
    if len(values) != 256:
        raise SystemExit("expected 256 original random table entries")
    output.parent.mkdir(parents=True, exist_ok=True)
    generated = output.parent / "original_movement.inc"
    generated.write_text(
        "/* Generated from verified id-Software/wolf3d sources. */\n"
        + "static const unsigned char rndtable[256] = {" + ",".join(values) + "};\n"
        + tables + "\n" + movement
    )
    bridge = pathlib.Path(__file__).with_name("bridge.c")
    subprocess.run([cc, "-std=c99", "-O2", "-Wall", "-Wextra",
                    "-Wno-unused-variable", "-I", str(output.parent),
                    str(bridge), "-o", str(output)], check=True)
    print(f"Compiled original TryWalk/SelectChaseDir/SelectDodgeDir/SelectRunDir: {output}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--cc", default="cc")
    args = parser.parse_args()
    build(args.source.resolve(), args.output.resolve(), args.cc)
