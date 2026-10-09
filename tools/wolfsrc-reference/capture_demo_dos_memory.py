#!/usr/bin/env python3
"""Run a supplied native recording in the user's registered DOS executable.

Only private copies of the supplied data are changed. By default this replaces
graphics demo chunks; --map-fixture explicitly replaces one map as well. The
executable and all remaining graphics chunks retain their supplied bytes. Reads are limited
to this tool's DOSBox child. Memory snapshots are diagnostic evidence; this tool
does not yet assert gameplay parity or infer unknown DOS symbol addresses.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import time

from capture_demo_dos import press_enter


REGISTERED_EXE_SHA256 = '2a969a97bc644d04be030d0d08bfda5477179d60f5dbfcc04616a18fa083ce77'


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def encode_demo(raw, dictionary):
    nodes = [struct.unpack_from('<HH', dictionary, i*4) for i in range(255)]
    codes = {}

    def visit(node, bits):
        for bit, child in enumerate(nodes[node]):
            if child < 256:
                codes[child] = bits + [bit]
            else:
                visit(child-256, bits + [bit])

    visit(254, [])
    encoded = bytearray(struct.pack('<I', len(raw)))
    value = shift = 0
    for byte in raw:
        for bit in codes[byte]:
            value |= bit << shift
            shift += 1
            if shift == 8:
                encoded.append(value)
                value = shift = 0
    if shift:
        encoded.append(value)
    return encoded


def replace_map_fixture(game, index, fixture):
    words = list(map(int, fixture.read_text().split()))
    if len(words) != 8195 or words[:3] != [64,64,3] or any(v < 0 or v > 65535 for v in words[3:]):
        raise ValueError('map fixture must contain width64 height64 difficulty3 and two4096-word planes')
    head = bytearray((game/'MAPHEAD.WL6').read_bytes())
    maps = bytearray((game/'GAMEMAPS.WL6').read_bytes())
    tag = struct.unpack_from('<H',head)[0]
    starts, lengths = [], []
    for plane in (words[3:4099],words[4099:8195],[0]*4096):
        rlew = [8192]
        for word in plane:
            rlew.extend([tag,1,word] if word == tag else [word])
        packed = bytearray(struct.pack('<H',len(rlew)*2))
        for word in rlew:
            if word >> 8 in (0xa7,0xa8):
                packed.extend((0,word>>8,word&255))
            else:
                packed.extend(struct.pack('<H',word))
        starts.append(len(maps)); lengths.append(len(packed)); maps.extend(packed)
    struct.pack_into('<I',head,2+index*4,len(maps))
    maps.extend(struct.pack('<IIIHHHHH16s',*starts,*lengths,64,64,b'DOS MEMORY TEST'))
    (game/'MAPHEAD.WL6').write_bytes(head)
    (game/'GAMEMAPS.WL6').write_bytes(maps)
    return {'path':str(fixture.resolve()),'sha256':digest(fixture.read_bytes()),'map_slot':index}


def prepare_game(data, demo, out, map_fixture=None):
    game = out / 'game'
    game.mkdir()
    originals = {}
    for path in data.iterdir():
        if path.name.upper() == 'WOLF3D.EXE' or path.suffix.upper() == '.WL6':
            raw = path.read_bytes()
            originals[path.name.upper()] = digest(raw)
            (game / path.name.upper()).write_bytes(raw)
    if originals.get('WOLF3D.EXE') != REGISTERED_EXE_SHA256:
        raise ValueError('expected the verified registered Apogee v1.4 executable')
    raw = demo.read_bytes()
    if len(raw) < 7 or raw[0] >= 60 or int.from_bytes(raw[1:3], 'little') != len(raw) or (len(raw)-4) % 3:
        raise ValueError('invalid native demo recording')
    graph = (game / 'VGAGRAPH.WL6').read_bytes()
    head = (game / 'VGAHEAD.WL6').read_bytes()
    offsets = [int.from_bytes(head[i:i+3], 'little') for i in range(0, len(head), 3)]
    encoded = encode_demo(raw, (game / 'VGADICT.WL6').read_bytes())
    # The verified Apogee executable uses T_DEMO0..3 == 151..154.
    rebuilt, changed = bytearray(), []
    for chunk, start in enumerate(offsets[:-1]):
        if start == 0xffffff:
            changed.append(start)
            continue
        end = next(v for v in offsets[chunk+1:] if v != 0xffffff)
        changed.append(len(rebuilt))
        rebuilt.extend(encoded if 151 <= chunk <= 154 else graph[start:end])
    changed.append(len(rebuilt))
    (game / 'VGAGRAPH.WL6').write_bytes(rebuilt)
    (game / 'VGAHEAD.WL6').write_bytes(b''.join(v.to_bytes(3, 'little') for v in changed))
    provenance = {'exe_sha256': REGISTERED_EXE_SHA256, 'demo_sha256': digest(raw),
                  'demo_file': str(demo.resolve()), 'map': raw[0], 'commands': (len(raw)-4)//3,
                  'original_files': originals, 'replaced_demo_chunks': [151, 152, 153, 154],
                  'patched_files': {name: digest((game/name).read_bytes()) for name in ('VGAHEAD.WL6', 'VGAGRAPH.WL6')}}
    if map_fixture:
        provenance['map_fixture'] = replace_map_fixture(game, raw[0], map_fixture)
        provenance['patched_files'].update({name:digest((game/name).read_bytes()) for name in ('MAPHEAD.WL6','GAMEMAPS.WL6')})
    (out / 'input-manifest.json').write_text(json.dumps(provenance, indent=2)+'\n')
    return game, provenance


def find_state(process, memory, pattern):
    for line in Path(f'/proc/{process.pid}/maps').read_text().splitlines():
        region, permissions, *_ = line.split()
        start, end = (int(value, 16) for value in region.split('-'))
        if not permissions.startswith('rw') or not 100000 < end-start < 64000000:
            continue
        try:
            memory.seek(start)
            index = memory.read(end-start).find(pattern)
        except OSError:
            continue  # A child mapping may change between /maps and /mem.
        if index >= 0:
            return start+index, start, end
    return None


def capture(args, out, game, provenance):
    config = out / 'dosbox.conf'
    config.write_text(f'''[sdl]
fullscreen=false
output=surface
[dosbox]
machine=vgaonly
memsize=16
[cpu]
cycles=fixed 20000
[mixer]
nosound=true
[sblaster]
sbtype=none
[autoexec]
mount c "{game}"
c:
wolf3d
''')
    pattern = struct.pack('<hhiiihhhhhhh', 3, provenance['map'], 0, 0, 40000, 3, 100, 8, 0, 1, 1, 1)
    last, found, dumped = None, None, set()
    acknowledged, screenshots = set(), set()
    zero_phase = None
    with (out/'dosbox.log').open('w') as log, (out/'gamestate.jsonl').open('w') as trace:
        process = subprocess.Popen([str(args.dosbox), '-conf', str(config)], env={**os.environ, 'SDL_AUDIODRIVER': 'dummy'}, stdout=log, stderr=log)
        try:
            start = time.monotonic()
            with open(f'/proc/{process.pid}/mem', 'rb', buffering=0) as memory:
                while process.poll() is None and time.monotonic()-start < args.seconds:
                    elapsed = time.monotonic()-start
                    for when in (3, 8, 60):
                        if elapsed > when and when not in screenshots:
                            if shutil.which('import'):
                                subprocess.run(['import', '-window', 'root', str(out/f'screen-{when:02d}s.png')], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                            screenshots.add(when)
                    # Registered v1.4 requires both the sign-on acknowledgement
                    # and NonShareware's separate IN_Ack before the timed intro.
                    for when in (5, 9):
                        if elapsed > when and when not in acknowledged:
                            press_enter()
                            acknowledged.add(when)
                    if found is None:
                        found = find_state(process, memory, pattern)
                        if found:
                            (out/'memory-location.json').write_text(json.dumps({'gamestate_host_address': found[0], 'mapping_start': found[1], 'mapping_end': found[2]}, indent=2)+'\n')
                    if found:
                        address, region_start, region_end = found
                        memory.seek(address)
                        raw = memory.read(66)
                        state = {'time_count': struct.unpack_from('<i', raw, 52)[0],
                                 'map': struct.unpack_from('<h', raw, 2)[0],
                                 'health': struct.unpack_from('<h', raw, 18)[0],
                                 'ammo': struct.unpack_from('<h', raw, 20)[0],
                                 'score': struct.unpack_from('<i', raw, 8)[0],
                                 'kills': struct.unpack_from('<h', raw, 44)[0]}
                        if state != last:
                            trace.write(json.dumps({'elapsed': time.monotonic()-start, **state})+'\n')
                            trace.flush()
                            last = state
                        tic = state['time_count']
                        # Preserve startup phases independently of TimeCount:
                        # SetupGameLevel and DrawPlayScreen both run at tic 0.
                        if tic == 0:
                            memory.seek(address-0x42e6+510)
                            area_alias = memory.read(2)
                            memory.seek(address-6)
                            player_pointer = memory.read(2)
                            phase = (area_alias, player_pointer)
                            if phase != zero_phase:
                                with (out/'startup-phases.jsonl').open('a') as phases:
                                    phases.write(json.dumps({'elapsed':time.monotonic()-start,
                                        'time_count':tic, 'area_255_word':int.from_bytes(area_alias,'little'),
                                        'player_pointer':int.from_bytes(player_pointer,'little')})+'\n')
                                zero_phase = phase
                        if tic in args.snapshot_tics and tic not in dumped:
                            base = max(region_start, address-0x80000)
                            end = min(region_end, address+0x20000)
                            memory.seek(base)
                            (out/f'memory-tic-{tic:05d}.bin').write_bytes(memory.read(end-base))
                            (out/f'memory-tic-{tic:05d}.json').write_text(json.dumps({'snapshot_host_base':base, 'gamestate_offset':address-base, 'tic':tic},indent=2)+'\n')
                            dumped.add(tic)
                        if tic >= args.stop_tic and dumped:
                            break
                    time.sleep(.002 if found else .05)
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=10)
    result = {'status':'captured' if found else 'no_demo_state_found', 'last_state':last, 'snapshot_tics':sorted(dumped),
              'parity_asserted':False, 'exe_sha256':REGISTERED_EXE_SHA256}
    (out/'result.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result))
    return 0 if found else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--dosbox', type=Path, required=True)
    parser.add_argument('--data', type=Path, required=True)
    parser.add_argument('--demo-file', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--map-fixture', type=Path, help='explicitly replace the demo map using a raw reference-input map header and planes')
    parser.add_argument('--seconds', type=float, default=90)
    parser.add_argument('--stop-tic', type=int, default=300)
    parser.add_argument('--snapshot-tics', type=int, nargs='+', default=[0,4,128,176,256,260,264,268,272,276,280])
    args = parser.parse_args()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    game, provenance = prepare_game(args.data, args.demo_file, out, args.map_fixture)
    return capture(args, out, game, provenance)


if __name__ == '__main__':
    raise SystemExit(main())
