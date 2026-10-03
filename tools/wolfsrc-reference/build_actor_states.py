#!/usr/bin/env python3
"""Compile original DoActor and state tables, with recorded callback invocations."""
import argparse
import hashlib
import pathlib
import re
import subprocess

from build_demo_player import function, SOURCE_REVISION
from build_demo_start import SOURCE_HASHES


def build(source, output, cc):
    hashes = {name: SOURCE_HASHES[name] for name in ('WL_DEF.H', 'WL_ACT2.C')}
    hashes['WL_PLAY.C'] = 'f8ef33eea485c5b9cd91c1167f431c058190148aaaf24296e1b7656e8d9cdf31'
    texts = {}
    for name, expected in hashes.items():
        raw = (source / 'WOLFSRC' / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise SystemExit(f'{name}: source differs from pinned revision {SOURCE_REVISION}')
        texts[name] = raw.decode('latin1')
    header = texts['WL_DEF.H']
    sprites = header[header.rfind('enum', 0, header.index('SPR_DEMO')):]
    sprites = sprites[:sprites.index('};') + 2]
    actor = texts['WL_ACT2.C']
    states = re.findall(r'statetype\s+(s_\w+)\s*=\s*\{[^;]+;', actor)
    names = [name for name in states if re.fullmatch(
        r's_(?:grd|ofc|ss|mut|dog|boss)(?:stand|path\w*|chase\w*|shoot\w*|pain\w*|jump\w*)', name)]
    definitions = '\n'.join(re.search(r'statetype\s+' + name + r'\s*=\s*\{[^;]+;', actor).group() for name in names)
    output.parent.mkdir(parents=True, exist_ok=True)
    (output.parent / 'original_actor_states.inc').write_text(
        '/* Generated from verified original source. */\n' + sprites + '\n'
        + '\n'.join('statetype ' + name + ';' for name in names) + '\n'
        + definitions + '\n' + function(texts['WL_PLAY.C'], 'DoActor') + '\n')
    subprocess.run([cc, '-std=c99', '-O2', '-Wall', '-Wextra', '-I', str(output.parent),
                    str(pathlib.Path(__file__).with_name('actor_states_bridge.c')),
                    '-o', str(output)], check=True)
    print(f'Compiled original DoActor and {len(names)} states: {output}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--cc', default='cc')
    args = parser.parse_args()
    build(args.source.resolve(), args.output.resolve(), args.cc)
