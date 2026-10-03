#!/usr/bin/env python3
"""Capture the first bundled DOS demo's health/ammo and compare runtime artifacts.

Linux only. Reads the memory of the DOSBox child started by this tool; no other
process is inspected. The original executable and data must match bundled v1.4.
Run under Xvfb, or provide an X11 display. This takes about two minutes.
"""
import argparse
import ctypes
import hashlib
import json
import os
import pathlib
import shutil
import struct
import subprocess
import tempfile
import time

HASHES = {
    'WOLF3D.EXE': '75bd63f1db75be77a9dfd317144fec2a86d2a409dd3b6847a251a2565b646280',
    'AUDIOHED.WL1': '39351624ae6f8eef4b873e060c1a6f3e5ee7e81c4939c485275a89b145336338',
    'AUDIOT.WL1': '1e2c9ae30398a14c61a4ddd39aabaa0dbcc984cc4924a3e51df75574c259cfb9',
    'GAMEMAPS.WL1': 'a6a6654b342f2c027bcb22bfce0a41f9fc0063b775e9e4da0c771970e53e11aa',
    'MAPHEAD.WL1': '3458f661c9b875bca99ea22a7267771fad8f2a33c699ea732cf7c3322909bf8c',
    'VGADICT.WL1': '59878fec65f033b00dbb1240317d1793f5858213dc8013b4d68f9ac8b45b0c80',
    'VGAGRAPH.WL1': 'd5176f843c53415132db199c19f38591eaf3cedd35a4f7eb2698c1865d83030d',
    'VGAHEAD.WL1': 'f4cc800dc8444373092d4eaa5d6ab59d63d23a510a9d38b73ca9b3dbb700d18b',
    'VSWAP.WL1': '698f217257e2cbb951a4d110ba09140291f38d0121b3784d1d6be59c03a6b47b',
}
# Original 16-bit gametype layout in WL_DEF.H: difficulty/map, three longs,
# lives/health/ammo/keys, and three weapon enums. This identifies the fresh demo.
START_PATTERN = struct.pack('<hhiiihhhhhhh', 3, 0, 0, 0, 40000, 3, 100, 8, 0, 1, 1, 1)


def find_game_state(process, memory):
    for line in pathlib.Path(f'/proc/{process.pid}/maps').read_text().splitlines():
        region, permissions, *_ = line.split()
        start, end = (int(value, 16) for value in region.split('-'))
        if not permissions.startswith('rw') or not 100000 < end - start < 64000000:
            continue
        memory.seek(start)
        index = memory.read(end - start).find(START_PATTERN)
        if index >= 0:
            return start + index
    return None


def press_enter():
    x11, xtest = ctypes.CDLL('libX11.so.6'), ctypes.CDLL('libXtst.so.6')
    x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
    x11.XOpenDisplay.restype = ctypes.c_void_p
    x11.XKeysymToKeycode.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
    x11.XKeysymToKeycode.restype = ctypes.c_uint
    x11.XFlush.argtypes = [ctypes.c_void_p]
    x11.XCloseDisplay.argtypes = [ctypes.c_void_p]
    xtest.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
    display = x11.XOpenDisplay(os.environ['DISPLAY'].encode())
    if not display:
        raise SystemExit('cannot open DOSBox test display')
    key = x11.XKeysymToKeycode(display, 0xff0d)
    for pressed in (1, 0):
        xtest.XTestFakeKeyEvent(display, key, pressed, 0)
        x11.XFlush(display)
        time.sleep(.1)
    x11.XCloseDisplay(display)


def capture(args, out, game):
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
    samples = {}
    with (out / 'dosbox.log').open('w') as log:
        process = subprocess.Popen([str(args.dosbox), '-conf', str(config)],
                                   env={**os.environ, 'SDL_AUDIODRIVER': 'dummy'}, stdout=log, stderr=log)
        try:
            start = time.monotonic()
            address, last, death_at = None, None, None
            acknowledged = False
            with open(f'/proc/{process.pid}/mem', 'rb', buffering=0) as memory, (out / 'gamestate.jsonl').open('w') as trace:
                while process.poll() is None and time.monotonic() - start < 130:
                    elapsed = time.monotonic() - start
                    if elapsed > 5 and not acknowledged:
                        press_enter()
                        acknowledged = True
                    if address is None and elapsed > 40:
                        address = find_game_state(process, memory)
                    if address is not None:
                        memory.seek(address)
                        data = memory.read(66)
                        state = {
                            'time_count': struct.unpack_from('<i', data, 52)[0],
                            'health': struct.unpack_from('<h', data, 18)[0],
                            'ammo': struct.unpack_from('<h', data, 20)[0],
                            'score': struct.unpack_from('<i', data, 8)[0],
                        }
                        if state != last:
                            trace.write(json.dumps({'elapsed': elapsed, **state}) + '\n')
                            trace.flush()
                            last = state
                        # TimeCount advances after rendering. Its first new value
                        # identifies the completed command; health can change
                        # during the next command before the counter advances.
                        if state['time_count']:
                            samples.setdefault(state['time_count'], state)
                        if state['time_count'] > 4000 and state['health'] == 0 and death_at is None:
                            death_at = elapsed
                        if death_at is not None and elapsed - death_at > 1:
                            break
                    time.sleep(.01)
        finally:
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=10)
    return samples


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--dosbox', type=pathlib.Path, required=True)
    parser.add_argument('--data', type=pathlib.Path, required=True)
    parser.add_argument('--runtime', type=pathlib.Path, required=True)
    parser.add_argument('--out', type=pathlib.Path, required=True)
    args = parser.parse_args()
    args.dosbox = args.dosbox.resolve()
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    for name, expected in HASHES.items():
        if hashlib.sha256((args.data / name).read_bytes()).hexdigest() != expected:
            raise SystemExit(f'{name}: expected bundled shareware v1.4')
    # A fresh game directory prevents an existing config changing the viewport.
    with tempfile.TemporaryDirectory(prefix='dos-game-', dir=out) as directory:
        game = pathlib.Path(directory)
        for path in args.data.iterdir():
            if path.name.upper() == 'WOLF3D.EXE' or path.suffix.upper() == '.WL1':
                shutil.copyfile(path, game / path.name)
        samples = capture(args, out, game)
    reference = [json.loads(line) for line in (args.runtime / 'reference-render.jsonl').read_text().splitlines()]
    if not reference or not reference[-1]['render']['died']:
        raise SystemExit('runtime reference must end at original death')
    for row in reference:
        command = row['command']
        state = samples.get((command + 1) * 4)
        if state is None:
            raise SystemExit(f'missing DOS sample after command {command}')
        for key in ('health', 'ammo'):
            if state[key] != row['render'][key]:
                raise SystemExit(f'command {command} {key}: DOS={state[key]}, reference={row["render"][key]}')
    if max(samples, default=0) != len(reference) * 4:
        raise SystemExit('DOS playback and reference terminal tic counts differ')
    terminal = samples[max(samples)]
    result = {'status': 'success', 'matched_commands': len(reference),
              'matched_tics': len(reference) * 4, 'terminal': 'death', **terminal,
              'verified_files': HASHES}
    (out / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
