#!/usr/bin/env python3
"""Compile verified original actor spawning and player face RNG routines."""
import argparse
import hashlib
import pathlib
import re
import subprocess

from build_demo_player import function, SOURCE_REVISION

SOURCE_HASHES = {
    'WL_STATE.C': '64f0f71a2e59f6c4077e6a03cebcdfff697ec8389acc665dfb75105c3a8fbbcc',
    'WL_ACT2.C': '16374a70218dceec093a83e7610cac201da0cf371cb14768f27ddc41836c24b0',
    'WL_GAME.C': 'adf9b84852a04bfbd27fda62b8337dd85435562e8aa563907d0ade2e43ff7423',
    'WL_ACT1.C': '0f0cee8d0025002c46671c49c7752f33b932e8501a31996c0727532761887a75',
    'WL_AGENT.C': '7ce0393c1a156664e94e3aad1dc36c147f6cf7f99c207643979adedaf2f3c82c',
    'WL_DEF.H': '2540f10aa19962b0016bf5afbcba432e46fc543be459af6e3f597dcff03a581c',
    'ID_US_A.ASM': 'bc2d15e24be9cf9ad3bbd779b520560f9a52c3e4e51e2e6e122e6bfc169afa7c',
}


def build(source, output, cc):
    texts = {}
    for name, expected in SOURCE_HASHES.items():
        raw = (source / 'WOLFSRC' / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise SystemExit(f'{name}: source differs from pinned revision {SOURCE_REVISION}')
        texts[name] = raw.decode('ascii')
    header = texts['WL_DEF.H']
    sprites = header[header.rfind('enum', 0, header.index('SPR_DEMO')):]
    sprites = sprites[:sprites.index('};') + 2]
    enemies = header[header.index('typedef enum {', header.index('#define NUMENEMIES')):]
    enemies = enemies[:enemies.index('} enemy_t;') + len('} enemy_t;')]
    actor = texts['WL_ACT2.C']
    hitpoints = actor[actor.index('int\tstarthitpoints'):actor.index('void\tT_Path')]
    # Keep the complete original table, excluding declarations following it.
    hitpoints = hitpoints[:re.search(r'}\s*;', hitpoints).end()]
    states = re.findall(r'statetype\s+(s_\w+)\s*=\s*\{[^;]+;', actor)
    names = [name for name in states if re.fullmatch(r's_(?:grd|ofc|ss|mut)(?:stand|path\w*)|s_dogpath\w*|s_bossstand|s_grddie4', name)]
    declarations = '\n'.join('statetype ' + name + ';' for name in names)
    definitions = '\n'.join(re.search(r'statetype\s+' + name + r'\s*=\s*\{[^;]+;', actor).group() for name in names)
    asm = texts['ID_US_A.ASM']
    values = re.findall(r'\b\d+\b', asm[asm.index('rndtable db'):asm.index('PUBLIC\trndtable')])
    if len(values) != 256:
        raise SystemExit('expected 256 original random table entries')
    routines = '\n\n'.join(function(texts[file], name) for file, name in (
        ('WL_STATE.C', 'SpawnNewObj'), ('WL_ACT2.C', 'SpawnStand'),
        ('WL_ACT2.C', 'SpawnPatrol'), ('WL_ACT2.C', 'SpawnDeadGuard'),
        ('WL_ACT2.C', 'SpawnBoss'), ('WL_ACT1.C', 'SpawnDoor'),
        ('WL_GAME.C', 'ScanInfoPlane'), ('WL_AGENT.C', 'UpdateFace')))
    # Preserve tagged-pointer values without truncating modern host addresses.
    routines = routines.replace('(unsigned)actorat[tilex][tiley] = doornum | 0x80;',
                                'actorat[tilex][tiley] = (objtype *)(uintptr_t)(doornum | 0x80);')
    # DOS unsigned is a 16-bit map word; host unsigned would change strides.
    routines = re.sub(r'\bunsigned\b', 'uint16_t', routines)
    output.parent.mkdir(parents=True, exist_ok=True)
    (output.parent / 'original_demo_start.inc').write_text(
        '/* Generated from verified original source. */\n' + sprites + '\n' + enemies
        + '\nstatic const unsigned char rndtable[256] = {' + ','.join(values) + '};\n'
        + hitpoints + '\n' + declarations + '\n' + definitions + '\n' + routines + '\n')
    subprocess.run([cc, '-std=c99', '-O2', '-Wall', '-Wextra', '-Wno-unused-variable',
                    '-Wno-implicit-fallthrough', '-Wno-switch', '-I', str(output.parent),
                    str(pathlib.Path(__file__).with_name('demo_start_bridge.c')),
                    '-o', str(output)], check=True)
    print(f'Compiled original ScanInfoPlane/spawning/UpdateFace: {output}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--cc', default='cc')
    args = parser.parse_args()
    build(args.source.resolve(), args.output.resolve(), args.cc)
