#!/usr/bin/env python3
"""Compile verified original player movement routines for demo comparisons."""
import argparse
import hashlib
import pathlib
import re
import subprocess

SOURCE_REVISION = "05167784ef009d0d0daefe8d012b027f39dc8541"
SOURCE_HASHES = {
    "WL_AGENT.C": "7ce0393c1a156664e94e3aad1dc36c147f6cf7f99c207643979adedaf2f3c82c",
    "WL_MAIN.C": "73a69d37497976c70031326b0ead1748a4bec330b427899393830260bdde0a00",
    "WL_DRAW.C": "669af4d388fb7f1b46ef6ab5689d1c6e2d3b17d2aaf964a87fdf54b388b453fb",
}


def function(text, name):
    # Strip comments before counting braces; keep the C function body itself.
    clean = re.sub(r"/\*.*?\*/|//[^\n]*", lambda m: ' ' * len(m.group()), text, flags=re.S)
    match = re.search(r"(?:void|boolean)\s+" + name + r"\s*\([^)]*\)\s*\{", clean)
    if not match:
        raise SystemExit(f"missing original function {name}")
    depth = 1
    pos = match.end()
    while depth:
        depth += (clean[pos] == '{') - (clean[pos] == '}')
        pos += 1
    return text[match.start():pos]


def build(source, output, cc):
    texts = {}
    for name, expected in SOURCE_HASHES.items():
        raw = (source / 'WOLFSRC' / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise SystemExit(f"{name}: source differs from pinned revision {SOURCE_REVISION}")
        texts[name] = raw.decode('ascii')
    routines = '\n\n'.join(function(texts['WL_AGENT.C'], name) for name in
                           ('ControlMovement', 'TryMove', 'ClipMove', 'Thrust'))
    # The source's tagged wall pointers must be compared numerically on hosts.
    routines = routines.replace('check<objlist', '(uintptr_t)check<(uintptr_t)objlist')
    routines = routines.replace('check > objlist', '(uintptr_t)check > (uintptr_t)objlist')
    tables = function(texts['WL_MAIN.C'], 'BuildTables')
    # Only the sine table is used by movement. Fine tangents belong to rendering.
    tables = tables[tables.index('  angle = 0;'):]
    output.parent.mkdir(parents=True, exist_ok=True)
    (output.parent / 'original_player_movement.inc').write_text(
        '/* Generated from verified original source. */\n'
        + 'static void BuildMovementTables(void) { int i; float angle,anglestep; fixed value;\n'
        + tables + '\n' + routines + '\n')
    bridge = pathlib.Path(__file__).with_name('demo_player_bridge.c')
    subprocess.run([cc, '-std=c99', '-O2', '-Wall', '-Wextra', '-Wno-unused-variable',
                    '-I', str(output.parent), str(bridge), '-lm', '-o', str(output)], check=True)
    print(f"Compiled original ControlMovement/TryMove/ClipMove/Thrust: {output}")


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--cc', default='cc')
    args = parser.parse_args()
    build(args.source.resolve(), args.output.resolve(), args.cc)
