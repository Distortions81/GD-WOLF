#!/usr/bin/env python3
"""Validate captured registered-DOS data layout and report source-memory aliases.

The player object locates DS using its actual near pointer. Door structures and
the complete initial wall mask validate the relevant linker-layout relationships
against map data and the independent C snapshot. No host-C layout is assumed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess

from capture_demo_dos_memory import REGISTERED_EXE_SHA256


def u16(data, offset):
    return struct.unpack_from('<H', data, offset)[0]


def i16(data, offset):
    return struct.unpack_from('<h', data, offset)[0]


def analyze(capture, reference_input, reference_state, reference_runtime=None):
    manifest = json.loads((capture/'input-manifest.json').read_text())
    if manifest['exe_sha256'] != REGISTERED_EXE_SHA256:
        raise ValueError('unverified executable profile')
    words = list(map(int, reference_input.read_text().split()))
    if words[:3] != [64, 64, 3]:
        raise ValueError('expected a 64x64 hard-difficulty reference map')
    walls, info = words[3:4099], words[4099:8195]
    player_tile = next(i for i, value in enumerate(info) if 19 <= value <= 22)
    px, py = player_tile % 64, player_tile // 64
    known_player = struct.pack('<iiHHB', px*65536+32768, py*65536+32768, px, py, (walls[player_tile]-107)&255)
    initial = json.loads(reference_state.read_text().splitlines()[0])['state']
    sample_path = capture/'memory-tic-00004.bin'
    sample = sample_path.read_bytes()
    metadata = json.loads(sample_path.with_suffix('.json').read_text())
    game = metadata['gamestate_offset']
    player_pointer = u16(sample, game-6)
    candidates = []
    start = 0
    while (position := sample.find(known_player, start)) >= 0:
        obj = position-16
        if u16(sample, obj) == 1 and u16(sample, obj+4) == 1 and sample[obj+8] == 4:
            candidates.append(obj-player_pointer)
        start = position+1
    if len(candidates) != 1:
        raise ValueError(f'player/DS identification was not unique: {candidates}')
    ds = candidates[0]
    # These relative offsets come from the pinned WOLF3D.MAP. They are checked
    # against every real door and all 4096 wall cells below, not simply trusted.
    symbols = {'gamestate':game, 'player':game-6, 'doorobjlist':game-0xf08,
               'actorat':game-0x4008, 'tilemap':game-0x2008,
               'areabyplayer':game-0x42e6, 'update':game-0x410c,
               'statobjlist':game-0xc86, 'spotvis':ds+0x9c48}
    doors = [(x,y,int(not (walls[y*64+x]&1)),(walls[y*64+x]-90)//2)
             for y in range(64) for x in range(64) if 90 <= walls[y*64+x] <= 101]
    for index, expected in enumerate(doors):
        found = struct.unpack_from('<BBHB', sample, symbols['doorobjlist']+index*10)
        if found != expected:
            raise ValueError(f'DOS door {index}: {found} != map {expected}')
    actual_walls = [int(bool(tile) and not 128 <= tile < 192)
                    for y in range(64) for x in range(64)
                    for tile in (sample[symbols['tilemap']+x*64+y],)]
    if actual_walls != initial['walls']:
        mismatch = next(i for i,(a,b) in enumerate(zip(actual_walls,initial['walls'])) if a != b)
        raise ValueError(f'DOS tilemap layout differs at {mismatch%64},{mismatch//64}')
    # Validate statobj_t's 8-byte layout and the near visspot pointer for every
    # populated initial object (including blocking/non-bonus statics).
    last_static = u16(sample, symbols['statobjlist']-2)
    static_count = (last_static-(symbols['statobjlist']-ds))//8
    if not 0 <= static_count <= 400:
        raise ValueError('invalid original static-object count')
    for index in range(static_count):
        at = symbols['statobjlist']+index*8
        x,y = sample[at:at+2]
        if u16(sample,at+2) != symbols['spotvis']-ds+x*64+y:
            raise ValueError(f'original static {index} visibility pointer disagrees with layout')
    samples = []
    for path in sorted(capture.glob('memory-tic-*.bin')):
        raw = path.read_bytes()
        tic = json.loads(path.with_suffix('.json').read_text())['tic']
        actors = []
        pointer = u16(raw, game-6)
        if pointer == player_pointer:
            pointer = u16(raw, ds+pointer+56)
            seen = set()
            while pointer and pointer not in seen:
                seen.add(pointer)
                at = ds+pointer
                state = u16(raw,at+6)
                actors.append({'pool_slot':(pointer-player_pointer)//60, 'class':u16(raw,at+4),
                               'active':bool(u16(raw,at)), 'tic_count':i16(raw,at+2),
                               'x':struct.unpack_from('<i',raw,at+16)[0], 'y':struct.unpack_from('<i',raw,at+20)[0],
                               'tile_x':u16(raw,at+24),'tile_y':u16(raw,at+26),'area':raw[at+28],
                               'flags':raw[at+8], 'health':i16(raw,at+44), 'shape':i16(raw,ds+state+2)})
                pointer = u16(raw,at+56)
        aliases = []
        for index in range(64,128):
            at = symbols['doorobjlist']+index*10
            x,y,vertical,lock,action,timer = struct.unpack_from('<BBHBxhh',raw,at)
            aliases.append({'index':index, 'tile_x':x,'tile_y':y,'vertical':vertical,
                            'lock':lock,'action':action,'timer':timer,
                            'statobj_byte_offset':at-symbols['statobjlist']})
        samples.append({'time_count':tic, 'area_255_word':u16(raw,symbols['areabyplayer']+510),
                        'update_values':sorted(set(raw[symbols['update']:symbols['update']+260])),
                        'actors':actors, 'door_aliases':aliases,
                        'snapshot_sha256':hashlib.sha256(raw).hexdigest()})
    result = {'status':'layout_verified', 'exe_sha256':REGISTERED_EXE_SHA256,
              'ds_snapshot_offset':ds, 'symbols':{name:at-ds for name,at in symbols.items()},
              'verified_doors':len(doors),'verified_wall_cells':4096,'verified_static_pointers':static_count,
              'object_size':60,'door_size':10,'static_size':8,
              'area_255_alias':'update[36:38]', 'samples':samples}
    if reference_runtime is not None:
        # Run only setup; no port-provided state enters this static check.
        run = subprocess.run([str(reference_runtime.resolve())], input=' '.join(map(str,words[:8195]))+'\n',
            text=True, capture_output=True, check=True, env={**os.environ,
                'GDWOLF_DEMO_STATICS':'1', 'GDWOLF_DEMO_SOUND_MODE':'off',
                'GDWOLF_DEMO_MEMORY_PROFILE':'registered-apogee-v1.4-2a969a97',
                'GDWOLF_DEMO_REGISTERED':'1','GDWOLF_DEMO_MAP_INDEX':str(manifest['map'])})
        state = json.loads(run.stdout)
        for index, door in enumerate(state['doors']):
            actual_action = i16(sample,symbols['doorobjlist']+index*10+6)
            if actual_action != door['action']:
                raise ValueError(f'original source/DOS door {index} action: {actual_action} != {door["action"]}')
        if len(state['statics']) != static_count:
            raise ValueError('original source/DOS initial static count differs')
        for index, obj in enumerate(state['statics']):
            expected = (obj['x'],obj['y'],symbols['spotvis']-ds+obj['x']*64+obj['y'],obj['shape'],obj['flags'],obj['item'])
            actual = struct.unpack_from('<BBHhBB',sample,symbols['statobjlist']+index*8)
            if actual != expected:
                raise ValueError(f'original source/DOS static {index}: {actual} != {expected}')
        result['source_static_verification'] = {'status':'passed', 'slots':static_count,
            'reference_sha256':hashlib.sha256(reference_runtime.read_bytes()).hexdigest(),
            'reference_input_sha256':hashlib.sha256(reference_input.read_bytes()).hexdigest()}
        result['source_door_verification'] = {'status':'passed', 'doors':len(state['doors']),
            'action_values':{'open':0,'closed':1,'opening':2,'closing':3}}
    (capture/'memory-analysis.json').write_text(json.dumps(result,indent=2)+'\n')
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--capture',type=Path,required=True)
    parser.add_argument('--reference-input',type=Path,required=True)
    parser.add_argument('--reference-state',type=Path,required=True)
    parser.add_argument('--reference-runtime',type=Path)
    args=parser.parse_args()
    result=analyze(args.capture,args.reference_input,args.reference_state,args.reference_runtime)
    print(json.dumps({k:v for k,v in result.items() if k!='samples'}))


if __name__=='__main__':
    main()
