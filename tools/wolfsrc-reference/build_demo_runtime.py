#!/usr/bin/env python3
"""Compile original demo runtime routines with floor visibility audited separately against original x86."""
import argparse
import hashlib
import pathlib
import re
import subprocess

from build_demo_player import function, SOURCE_REVISION, SOURCE_HASHES as RENDER_HASHES
from build_demo_start import SOURCE_HASHES
from build_demo_audio import SOURCE_HASHES as AUDIO_HASHES, build_include, death_scream


def build(source, output, cc):
    hashes = dict(SOURCE_HASHES)
    hashes.update(RENDER_HASHES)
    hashes.update(AUDIO_HASHES)
    hashes['WL_PLAY.C'] = 'f8ef33eea485c5b9cd91c1167f431c058190148aaaf24296e1b7656e8d9cdf31'
    hashes['ID_VH.C'] = '787cc1bf483432b9b922beb69356aa134224dd1bbedd9fb8bcbaae19da8d863b'
    hashes['ID_VH_A.ASM'] = '90fd0dacddd015110a50ed2df78d1ade30c46b13c5c571b6480ebd686f95b96f'
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
    enums += '\n' + re.search(r'enum\s*\{dr_open,dr_closed,dr_opening,dr_closing}', header).group() + ';\n'
    hitpoints = actor[actor.index('int\tstarthitpoints'):]
    hitpoints = hitpoints[:re.search(r'}\s*;', hitpoints).end()]
    statinfo = re.search(r'statinfo\[\] =\s*(\{.*?});', texts['WL_ACT1.C'], re.S).group(1)
    actor_prefixes = ('grd|ofc|ss|mut|dog|boss|gretel|schabb|gift|fat|fake|mecha|hitler|'
                      'blinky|inky|pinky|clyde|needle|rocket|smoke|boom|fire|bj|deathcam')
    definitions = [match.group() for match in re.finditer(r'statetype\s+(s_\w+)\s*=\s*\{[^;]+;', actor)
                   if re.fullmatch(r's_(?:' + actor_prefixes + r')\w*', match.group(1))]
    definitions += [match.group() for match in re.finditer(
        r'statetype\s+(s_(?:player|attack))\s*=\s*\{[^;]+;', texts['WL_AGENT.C'])]
    names = [re.search(r'statetype\s+(\w+)', definition).group(1) for definition in definitions]
    asm = texts['ID_US_A.ASM']
    randoms = re.findall(r'\b\d+\b', asm[asm.index('rndtable db'):asm.index('PUBLIC\trndtable')])
    if len(randoms) != 256:
        raise SystemExit('expected 256 original random bytes')
    routine_groups = (
        ('WL_STATE.C', ('SpawnNewObj', 'NewState', 'MoveObj', 'CheckLine', 'CheckSight',
                        'FirstSighting', 'SightPlayer', 'DamageActor', 'KillActor')),
        ('WL_ACT2.C', ('SpawnStand', 'SpawnPatrol', 'SpawnDeadGuard', 'SpawnBoss', 'SpawnGretel',
                       'SpawnSchabbs', 'SpawnGift', 'SpawnFat', 'SpawnFakeHitler', 'SpawnHitler', 'SpawnGhosts',
                       'T_Schabb', 'T_Gift', 'T_Fat', 'T_Fake', 'T_Ghosts',
                       'T_SchabbThrow', 'T_GiftThrow', 'T_FakeFire', 'T_Projectile', 'ProjectileTryMove',
                       'A_Smoke', 'A_HitlerMorph', 'A_MechaSound', 'A_Slurpie',
                       'SpawnBJVictory', 'T_BJRun', 'T_BJJump', 'T_BJYell', 'T_BJDone',
                       'CheckPosition', 'A_StartDeathCam',
                       'SelectPathDir', 'T_Path', 'T_Chase', 'T_DogChase', 'T_Stand',
                       'T_Shoot', 'T_Bite', 'A_DeathScream')),
        ('WL_ACT1.C', ('SpawnStatic', 'PlaceItemType', 'SpawnDoor', 'RecursiveConnect', 'ConnectAreas', 'InitAreas',
                       'OpenDoor', 'CloseDoor', 'OperateDoor', 'DoorOpen',
                       'DoorOpening', 'DoorClosing', 'MoveDoors', 'PushWall', 'MovePWalls')),
        ('WL_GAME.C', ('ScanInfoPlane',)),
        ('WL_AGENT.C', ('SpawnPlayer', 'TakeDamage', 'UpdateFace', 'CheckWeaponChange', 'GiveAmmo', 'GiveWeapon', 'HealSelf', 'GiveKey', 'GiveExtraMan', 'GivePoints', 'GetBonus', 'TryMove', 'ClipMove', 'Thrust', 'ControlMovement', 'GunAttack', 'KnifeAttack', 'Cmd_Use', 'Cmd_Fire', 'T_Attack', 'T_Player', 'VictoryTile', 'VictorySpin')),
        ('WL_PLAY.C', ('InitActorList', 'GetNewActor', 'RemoveObj', 'DoActor')))
    routines = '\n\n'.join(function(texts[file], name) for file, names_ in routine_groups for name in names_)
    # Preserve original door decisions; serialize only their writes when a
    # verified DOS alias points into statobjlist rather than the host array.
    for name in ('OpenDoor', 'CloseDoor'):
        original = function(texts['WL_ACT1.C'], name)
        adapted = original[:original.rfind('}')] + '\n reference_commit_door(door);\n}'
        if name == 'CloseDoor':
            adapted = re.sub(r'actorat\[(tilex(?:[+-]1)?)\]\[(tiley(?:[+-]1)?)\](?!\s*\n?\s*=)',
                             r'reference_actor_at(__func__, \1, \2)', adapted)
            adapted = adapted.replace('check->x', 'reference_object_coordinate(__func__, check, 0)')
            adapted = adapted.replace('check->y', 'reference_object_coordinate(__func__, check, 1)')
        routines = routines.replace(original, adapted)
    game_setup = function(texts['WL_GAME.C'], 'SetupGameLevel')
    ambush_start = game_setup.index('map = mapsegs[0];', game_setup.index('// take out the ambush markers'))
    ambush_end = game_setup.index('CA_LoadAllSounds ();', ambush_start)
    routines += ('\nstatic void ClearAmbushMarkers(void) { uint16_t *map, tile; int x,y;\n'
                 + game_setup[ambush_start:ambush_end] + '\n}\n')
    tables = state[state.index('dirtype opposite[9]'):state.index('void\tSpawnNewObj')]
    movement = state[state.index('#define CHECKDIAG'):state.index('void MoveObj (objtype *ob, long move)\n{')]
    routines = tables + '\n' + movement + '\n' + routines
    routines = routines.replace(function(actor, 'A_DeathScream'), death_scream(actor, cc))
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
    routines, count = re.subn(r'(void\s+SpawnStatic\s*\([^)]*\)\s*\{)',
        r'\1\n if (type < 0 || (size_t)type >= sizeof(statinfo)/sizeof(statinfo[0])) { fprintf(stderr, "unsupported original memory access: SpawnStatic type=%d outside statinfo; command=%d tile=(%d,%d)\\n", type, reference_command_index, tilex, tiley); exit(3); }', routines)
    if count != 1:
        raise SystemExit('expected one original SpawnStatic entry')
    # Keep the original 37-area bounds rather than allowing invalid original
    # byte area values to index a larger, silently zero-filled host array.
    routines = re.sub(r'areabyplayer\[([^\]]+)\](\s*=)?',
                      lambda m: ('areabyplayer[reference_validate_area_index(__func__, '
                                 + m[1] + ')]' + m[2]) if m[2] else
                                'reference_area_by_player(__func__, ' + m[1] + ')', routines)
    routines = re.sub(r'areaconnect\[([^\]]+)\]\[([^\]]+)\]',
                      r'areaconnect[reference_validate_area_index(__func__, \1)][reference_validate_area_index(__func__, \2)]', routines)
    routines = re.sub(r'doorposition\[([^\]]+)\]',
                      r'doorposition[reference_validate_door_position(__func__, \1)]', routines)
    routines, count = re.subn(r'(void\s+SpawnDoor\s*\([^)]*\)\s*\{)',
                             r'\1\n reference_validate_door_index("SpawnDoor", doornum);', routines)
    if count != 1:
        raise SystemExit('expected one original SpawnDoor entry')
    routines = routines.replace('check<objlist', '(uintptr_t)check<256')
    routines = routines.replace('check > objlist', '(uintptr_t)check >= (uintptr_t)objects')
    routines = re.sub(r'void\s+ControlMovement\s*\(', 'void OriginalControlMovement (', routines)
    routines = re.sub(r'void\s+UpdateFace\s*\(', 'void OriginalUpdateFace (', routines)
    for attack in ('GunAttack', 'KnifeAttack'):
        routines = re.sub(r'(void\s+' + attack + r'\s*\([^)]*\)\s*\{)', r'\1\n reference_shots++;', routines)
    # Cmd_Use mistakes moving pushwall markers (0xC0 | texture) for doors.
    # The resulting door index may exceed the original 64-element array.
    # The verified DOS profile reconstructs those reads from serialized
    # original statics. Unknown profiles and unverified writes still fail.
    routines = routines.replace('doorobjlist[door]', '(*reference_door(__func__, door))')
    for door_routine in ('OpenDoor', 'CloseDoor', 'OperateDoor'):
        routines, count = re.subn(
            r'(void\s+' + door_routine + r'\s*\(int\s+door\)\s*\{)',
            r'\1\n reference_validate_door_index("' + door_routine + r'", door);', routines)
        if count != 1:
            raise SystemExit(f'expected one original {door_routine} entry')
    routines = re.sub(r'(void\s+TakeDamage\s*\([^)]*\)\s*\{)', r'\1\n player_damage += points;', routines)
    attackinfo = re.search(r'struct atkinf\s*\{.*?attackinfo\[4\]\[14\]\s*=\s*\{.*?\n};', texts['WL_AGENT.C'], re.S).group()
    render = '\n'.join(function(texts[file], name) for file, name in (
        ('WL_MAIN.C', 'BuildTables'), ('WL_MAIN.C', 'CalcProjection'),
        ('WL_DRAW.C', 'TransformActor'), ('WL_DRAW.C', 'TransformTile')))
    # The shared extractor handles void/boolean signatures; only the signature
    # marker is normalized, then restored to the original integer return type.
    rotate_source = re.sub(r'\bint\s+CalcRotate\s*\(', 'boolean CalcRotate (', texts['WL_DRAW.C'])
    rotate = function(rotate_source, 'CalcRotate').replace('boolean CalcRotate', 'int CalcRotate', 1)
    dirangles = re.search(r'int\s+dirangle\[9\]\s*=\s*\{[^}]+};', texts['WL_MAIN.C']).group()
    render += '\n' + dirangles + '\n' + rotate
    mark_source = re.sub(r'\bint\s+VW_MarkUpdateBlock\s*\(', 'boolean VW_MarkUpdateBlock (', texts['ID_VH.C'])
    mark = function(mark_source, 'VW_MarkUpdateBlock').replace('boolean VW_MarkUpdateBlock', 'int VW_MarkUpdateBlock', 1)
    render += '\n' + mark + '\n' + function(texts['WL_GAME.C'], 'DrawPlayBorder')
    for instruction in ('test\t[update+bx],1', 'mov\t[update+bx],0'):
        if instruction not in texts['ID_VH_A.ASM']:
            raise SystemExit('original dirty-block clear instruction missing')
    # Replace only the height idiv assembly; projection math stays original C.
    render, count = re.subn(r'asm\s+mov\s+ax,\[WORD PTR heightnumerator\].*?asm\s+mov\s+\[WORD PTR temp\+2\],dx',
                           'temp = (int16_t)(heightnumerator / (nx >> 8));', render, flags=re.S)
    if count != 2:
        raise SystemExit('expected two original projection height divisions')
    draw = function(texts['WL_DRAW.C'], 'DrawScaleds')
    draw_prefix = draw[:draw.index('// draw from back to front')]
    draw_prefix = draw_prefix[:draw_prefix.rfind('//')]
    render += '\n' + draw_prefix.replace('DrawScaleds', 'RefreshActorVisibility') + '\n}\n'
    # SPEAR branches are disabled. Declarations keep their switch references
    # available without substituting any executable Wolf3D state definition.
    references = set(re.findall(r'&(s_\w+)', routines + '\n'.join(definitions)))
    extra_states = references - set(names)
    prototypes = '\n'.join(re.match(r'(?:void|boolean)\s+\w+\s*\([^)]*\)',
                                    function(texts[file], name)).group() + ';'
                           for file, names_ in routine_groups
                           for name in names_)
    prototypes = re.sub(r'\bunsigned\b', 'uint16_t', prototypes)
    prototypes = prototypes.replace('void\t', 'void ')
    # Identify state families independently of the Go animation tables. Shapes
    # alone cannot distinguish path/chase states with different timing/think.
    state_kinds = {'stand': 0, 'path': 1, 'chase': 2, 'shoot': 3, 'jump': 4,
                   'pain': 5, 'die': 6, 'dead': 6, 'deathcam': 6, 'run': 9}
    state_classifier = ['static int reference_actor_state(statetype *state) {']
    for name in names:
        family = re.fullmatch(r's_(?:grd|ofc|ss|mut|dog|boss|gretel|schabb|gift|fat|fake|mecha|hitler|blinky|inky|pinky|clyde|bj)([a-z]+)\d*s?', name)
        if family and family[1] in state_kinds:
            state_kind = state_kinds[family[1]]
        elif re.fullmatch(r's_(?:needle|rocket|fire)\d*', name):
            state_kind = 7
        elif re.fullmatch(r's_(?:smoke|boom)\d+', name):
            state_kind = 8
        elif name in ('s_player', 's_attack', 's_deathcam'):
            continue
        else:
            raise SystemExit(f'unclassified original actor state: {name}')
        state_classifier.append(f'    if (state == &{name}) return {state_kind};')
    state_classifier.append('    Quit("unknown original actor state"); return -1;\n}')
    output.parent.mkdir(parents=True, exist_ok=True)
    build_include(texts, output.parent, cc)
    (output.parent / 'original_demo_runtime_types.inc').write_text(enums + '\n')
    (output.parent / 'original_demo_runtime.inc').write_text(
        '/* Generated from verified original source. */\n' + sprites + '\n' + prototypes
        + '\nstatic const unsigned char rndtable[256] = {' + ','.join(randoms) + '};\n'
        + hitpoints + '\n' + attackinfo + '\nstatic struct { int picnum; stat_t type; } statinfo[] = ' + statinfo + ';\n'
        + '\n'.join('statetype ' + name + ';' for name in names + sorted(extra_states)) + '\n'
        + '\n'.join(definitions) + '\n' + routines + '\n' + render + '\n'
        + 'static void reference_finish_tic(void) {'
        + re.search(r'gamestate\.TimeCount\s*\+=\s*tics;', function(texts['WL_PLAY.C'], 'PlayLoop')).group()
        + '}\n'
        + '\n'.join(state_classifier) + '\n')
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
