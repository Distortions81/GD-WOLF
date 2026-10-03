#!/usr/bin/env python3
"""Compile original demo runtime routines with shared floor visibility/use inputs."""
import argparse
import hashlib
import pathlib
import re
import subprocess

from build_demo_player import function, SOURCE_REVISION, SOURCE_HASHES as RENDER_HASHES
from build_demo_start import SOURCE_HASHES


def build(source, output, cc):
    hashes = dict(SOURCE_HASHES)
    hashes.update(RENDER_HASHES)
    hashes['WL_PLAY.C'] = 'f8ef33eea485c5b9cd91c1167f431c058190148aaaf24296e1b7656e8d9cdf31'
    texts = {}
    for name, expected in hashes.items():
        raw = (source / 'WOLFSRC' / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != expected:
            raise SystemExit(f'{name}: source differs from pinned revision {SOURCE_REVISION}')
        texts[name] = raw.decode('latin1')
    header, actor, state = texts['WL_DEF.H'], texts['WL_ACT2.C'], texts['WL_STATE.C']
    sprites = header[header.rfind('enum', 0, header.index('SPR_DEMO')):]
    sprites = sprites[:sprites.index('};') + 2]
    enums = '\n'.join(match.group() for match in re.finditer(
        r'typedef enum\s*\{[^}]+}\s*(?:classtype|stat_t|dirtype|enemy_t);', header))
    hitpoints = actor[actor.index('int\tstarthitpoints'):]
    hitpoints = hitpoints[:re.search(r'}\s*;', hitpoints).end()]
    statinfo = re.search(r'statinfo\[\] =\s*(\{.*?});', texts['WL_ACT1.C'], re.S).group(1)
    definitions = [match.group() for match in re.finditer(r'statetype\s+(s_\w+)\s*=\s*\{[^;]+;', actor)
                   if re.fullmatch(r's_(?:grd|ofc|ss|mut|dog)\w*|s_boss(?:stand|chase\w*|shoot\w*|die\w*)', match.group(1))]
    names = [re.search(r'statetype\s+(\w+)', definition).group(1) for definition in definitions]
    asm = texts['ID_US_A.ASM']
    randoms = re.findall(r'\b\d+\b', asm[asm.index('rndtable db'):asm.index('PUBLIC\trndtable')])
    if len(randoms) != 256:
        raise SystemExit('expected 256 original random bytes')
    routines = '\n\n'.join(function(texts[file], name) for file, names_ in (
        ('WL_STATE.C', ('SpawnNewObj', 'NewState', 'MoveObj', 'CheckLine', 'CheckSight',
                        'FirstSighting', 'SightPlayer', 'DamageActor', 'KillActor')),
        ('WL_ACT2.C', ('SpawnStand', 'SpawnPatrol', 'SpawnDeadGuard', 'SpawnBoss',
                       'SelectPathDir', 'T_Path', 'T_Chase', 'T_DogChase', 'T_Stand',
                       'T_Shoot', 'T_Bite', 'A_DeathScream')),
        ('WL_ACT1.C', ('SpawnStatic', 'PlaceItemType', 'SpawnDoor', 'RecursiveConnect', 'ConnectAreas', 'InitAreas',
                       'OpenDoor', 'CloseDoor', 'OperateDoor', 'DoorOpen',
                       'DoorOpening', 'DoorClosing', 'MoveDoors', 'PushWall', 'MovePWalls')),
        ('WL_GAME.C', ('ScanInfoPlane',)),
        ('WL_AGENT.C', ('UpdateFace', 'CheckWeaponChange', 'GiveAmmo', 'GiveWeapon', 'HealSelf', 'GiveKey', 'GiveExtraMan', 'GetBonus', 'TryMove', 'ClipMove', 'Thrust', 'ControlMovement', 'GunAttack', 'KnifeAttack', 'Cmd_Fire', 'T_Attack')),
        ('WL_PLAY.C', ('DoActor',))) for name in names_)
    tables = state[state.index('dirtype opposite[9]'):state.index('void\tSpawnNewObj')]
    movement = state[state.index('#define CHECKDIAG'):state.index('void MoveObj (objtype *ob, long move)\n{')]
    routines = tables + '\n' + movement + '\n' + routines
    routines = routines.replace('(unsigned)actorat[tilex][tiley] = doornum | 0x80;',
                                'actorat[tilex][tiley] = (objtype *)(uintptr_t)(doornum | 0x80);')
    routines = re.sub(r'\(unsigned\)actorat\[tilex\]\[tiley\]\s*= door \| 0x80;',
                      'actorat[tilex][tiley] = (objtype *)(uintptr_t)(door | 0x80);', routines)
    routines = re.sub(r'\(unsigned\)(actorat\[[^\n]+?\]\[[^\n]+?\])\s*=\s*(tilemap\[[^\n]+?\]\[[^\n]+?\]\s*=\s*oldtile);',
                      r'\1 = (objtype *)(uintptr_t)(\2);', routines)
    routines = routines.replace('(unsigned)actorat[pwallx][pwally] = 0;', 'actorat[pwallx][pwally] = NULL;')
    routines = routines.replace('(unsigned)actorat[tilex][tiley] = 1;', 'actorat[tilex][tiley] = (objtype *)(uintptr_t)1;')
    routines = routines.replace('(unsigned)actorat', '(uintptr_t)actorat')
    routines = routines.replace('unsigned\ttemp;', 'uintptr_t\ttemp;')
    routines = re.sub(r'\bunsigned\b', 'uint16_t', routines)
    routines = routines.replace('check<objlist', '(uintptr_t)check<256')
    routines = routines.replace('check > objlist', '(uintptr_t)check >= (uintptr_t)objects')
    routines = re.sub(r'void\s+ControlMovement\s*\(', 'void OriginalControlMovement (', routines)
    routines = re.sub(r'void\s+UpdateFace\s*\(', 'void OriginalUpdateFace (', routines)
    for attack in ('GunAttack', 'KnifeAttack'):
        routines = re.sub(r'(void\s+' + attack + r'\s*\([^)]*\)\s*\{)', r'\1\n reference_shots++;', routines)
    attackinfo = re.search(r'struct atkinf\s*\{.*?attackinfo\[4\]\[14\]\s*=\s*\{.*?\n};', texts['WL_AGENT.C'], re.S).group()
    render = '\n'.join(function(texts[file], name) for file, name in (
        ('WL_MAIN.C', 'BuildTables'), ('WL_MAIN.C', 'CalcProjection'),
        ('WL_DRAW.C', 'TransformActor'), ('WL_DRAW.C', 'TransformTile')))
    # Replace only the height idiv assembly; projection math stays original C.
    render, count = re.subn(r'asm\s+mov\s+ax,\[WORD PTR heightnumerator\].*?asm\s+mov\s+\[WORD PTR temp\+2\],dx',
                           'temp = (int16_t)(heightnumerator / (nx >> 8));', render, flags=re.S)
    if count != 2:
        raise SystemExit('expected two original projection height divisions')
    draw = function(texts['WL_DRAW.C'], 'DrawScaleds')
    draw_prefix = draw[:draw.index('// draw from back to front')]
    draw_prefix = draw_prefix[:draw_prefix.rfind('//')]
    render += '\n' + draw_prefix.replace('DrawScaleds', 'RefreshActorVisibility') + '\n}\n'
    # Unsupported state families are never spawned, but their switch cases
    # remain compiled. Keep declarations for those unexecuted source branches.
    references = set(re.findall(r'&(s_\w+)', routines + '\n'.join(definitions)))
    extra_states = references - set(names)
    sounds = set(re.findall(r'\b[A-Z][A-Z0-9_]*SND\b', routines))
    prototypes = '\n'.join(re.match(r'(?:void|boolean)\s+\w+\s*\([^)]*\)',
                                    function(texts[file], name)).group() + ';'
                           for file, names_ in (
        ('WL_STATE.C', ('NewState', 'CheckLine', 'CheckSight', 'FirstSighting', 'SightPlayer', 'DamageActor', 'KillActor')),
        ('WL_ACT2.C', ('SelectPathDir', 'T_Path', 'T_Chase', 'T_DogChase', 'T_Stand', 'T_Shoot', 'T_Bite', 'A_DeathScream')))
                           for name in names_)
    prototypes = re.sub(r'\bunsigned\b', 'uint16_t', prototypes)
    output.parent.mkdir(parents=True, exist_ok=True)
    (output.parent / 'original_demo_runtime_types.inc').write_text(enums + '\n')
    (output.parent / 'original_demo_runtime.inc').write_text(
        '/* Generated from verified original source. */\n' + sprites + '\n' + prototypes
        + '\nstatic const unsigned char rndtable[256] = {' + ','.join(randoms) + '};\n'
        + '\n'.join('#define ' + sound + ' 0' for sound in sorted(sounds)) + '\n'
        + hitpoints + '\n' + attackinfo + '\nstatic struct { int picnum; stat_t type; } statinfo[] = ' + statinfo + ';\n'
        + '\n'.join('statetype ' + name + ';' for name in names + sorted(extra_states)) + '\n'
        + '\n'.join(definitions) + '\n' + routines + '\n' + render + '\n')
    subprocess.run([cc, '-std=c99', '-O2', '-Wall', '-Wextra', '-Wno-unused-variable',
                    '-Wno-implicit-fallthrough', '-Wno-switch', '-Wno-missing-field-initializers', '-I', str(output.parent),
                    str(pathlib.Path(__file__).with_name('demo_runtime_bridge.c')),
                    '-lm', '-o', str(output)], check=True)
    print(f'Compiled original demo runtime: {output}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--cc', default='cc')
    args = parser.parse_args()
    build(args.source.resolve(), args.output.resolve(), args.cc)
