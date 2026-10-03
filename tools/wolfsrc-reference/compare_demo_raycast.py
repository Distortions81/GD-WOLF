#!/usr/bin/env python3
"""Run verified original 16-bit ray traversal under QEMU and compare floor masks.

Only assembler syntax, segment placement, and omitted drawing calls are adapted.
The original two self-modifying short branches and all arithmetic execute as x86.
"""
import argparse
import hashlib
import pathlib
import re
import struct
import json
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--source', type=pathlib.Path, required=True)
parser.add_argument('--reference', type=pathlib.Path, required=True)
parser.add_argument('--trace', type=pathlib.Path, required=True)
parser.add_argument('--out', type=pathlib.Path, required=True)
args = parser.parse_args()
out = args.out.resolve()
out.mkdir(parents=True, exist_ok=True)
raw = (args.source / 'WOLFSRC' / 'WL_DR_A.ASM').read_bytes()
if hashlib.sha256(raw).hexdigest() != 'c417570e2cbd56c25731d8eae6684997dee432a698c31a2f64ccb74448d0a13e':
    raise SystemExit('WL_DR_A.ASM differs from pinned original source')
source = raw.decode('latin1')
source = source[source.index('PROC xpartialbyystep'):source.rindex('ENDP')]


def memory_operand(match):
    expression = match.group(1)
    size = 'WORD'
    if expression.startswith(('WORD ', 'BYTE ')):
        size, expression = expression.split(' ', 1)
    segment = ''
    if ':' in expression:
        segment, expression = expression.split(':', 1)
        segment += ':'
    return f'{size} PTR {segment}[{expression}]'


body = []
procedure = ''
for line in source.splitlines():
    instruction = line.split(';')[0].strip()
    if not instruction:
        continue
    if instruction.startswith('PROC'):
        procedure = instruction.split()[1]
        body.append(procedure + ':')
        continue
    if instruction.startswith(('ENDP', 'PUBLIC')):
        continue
    if instruction == 'EVEN':
        body.append('.align 2')
        continue
    instruction = instruction.replace('@@multpos', procedure + '_multpos')
    instruction = re.sub(r'\b([0-9][0-9a-fA-F]*)h\b', r'0x\1', instruction)
    # The original far tangent table starts at offset zero in its segment.
    instruction = instruction.replace('SEG finetangent', '0x1400')
    if re.match(r'call\s+FAR', instruction):
        body.append('nop')  # Wall drawing callbacks cannot change floor traversal.
        continue
    instruction = re.sub(r'call\s+NEAR\s+', 'call ', instruction)
    instruction = instruction.replace('retf', 'ret')  # Test entry uses a near call.
    instruction = re.sub(r'\[([^]]+)\]', memory_operand, instruction)
    # Opcode bytes are patched by the source for each ray quadrant.
    if re.fullmatch(r'jle\s+(horizentry|vertentry)', instruction):
        target = instruction.split()[1]
        instruction = f'.byte 0x7e\n.byte {target} - . - 1'
    body.append(instruction)

# Test-owned DOS data segment. Tile arrays retain original X-major indexing.
constants = {
    'viewwidth': 0, 'midangle': 2, 'focaltx': 4, 'focalty': 6,
    'viewx': 8, 'viewy': 12, 'xpartialdown': 16, 'xpartialup': 18,
    'ypartialdown': 20, 'ypartialup': 22, 'xstep': 24, 'ystep': 28,
    'yintercept': 32, 'xintercept': 36, 'xtile': 40, 'ytile': 42,
    'xtilestep': 44, 'ytilestep': 46, 'tilehit': 48, 'pixx': 50,
    'pwallpos': 52, 'tilemap': 0x1000, 'spotvis': 0x2000,
    'pixelangle': 0x3000, 'doorposition': 0x3300,
    'SCREENSEG': 0xa000, 'FINEANGLES': 3600, 'DEG90': 900,
    'DEG180': 1800, 'DEG270': 2700, 'DEG360': 3600,
    'OP_JLE': 0x7e, 'OP_JGE': 0x7d,
}
frames = [json.loads(line) for line in args.trace.read_text().splitlines()]
tables = json.loads(subprocess.check_output([str(args.reference.resolve()), '--render-tables']))
if not 0 < len(frames) < 65536:
    raise SystemExit('raycaster trace must contain 1..65535 frames')
# BIOS reads each 40-sector input record, then executes AsmRefresh and emits
# the 4096 visibility bytes through QEMU's debug port. No disk writes occur.
boot = '''.intel_syntax noprefix
.code16
.global _start
_start:
 ljmp 0, boot
boot:
 cli
 xor ax, ax
 mov ds, ax
 mov ss, ax
 mov sp, 0x7c00
 sti
 cld
 mov BYTE PTR [drive], dl
 mov si, OFFSET stage_packet
 mov ah, 0x42
 int 0x13
 jc failed
 jmp stage
failed:
 mov al, 0xff
 out 0xe9, al
 mov dx, 0xf4
 out dx, al
 hlt
drive: .byte 0
.align 2
stage_packet:
 .byte 16,0
 .word 32,0x7e00,0
 .quad 1
.org 510
 .word 0xaa55
stage:
 mov WORD PTR [remaining], FRAME_COUNT
frame_loop:
 xor ax, ax
 mov ds, ax
 mov dl, BYTE PTR [drive]
 mov si, OFFSET frame_packet
 mov ah, 0x42
 int 0x13
 jc failed
 mov ax, 0x1000
 mov ds, ax
 call AsmRefresh
 mov si, 0x2000
 mov cx, 4096
output_loop:
 lodsb
 out 0xe9, al
 loop output_loop
 xor ax, ax
 mov ds, ax
 add WORD PTR [frame_packet+8], 40
 adc WORD PTR [frame_packet+10], 0
 dec WORD PTR [remaining]
 jnz frame_loop
 mov dx, 0xf4
 mov al, 0x10
 out dx, al
 hlt
remaining: .word 0
.align 2
frame_packet:
 .byte 16,0
 .word 40,0,0x1000
 .quad 33
'''.replace('FRAME_COUNT', str(len(frames)))
assembly = boot + '\n' + '\n'.join(f'.equ {key}, {value}' for key, value in constants.items())
assembly += '\n' + '\n'.join(body) + '\n'
(out / 'raycast.s').write_text(assembly)
subprocess.run(['as', '--32', '-o', str(out / 'raycast.o'), str(out / 'raycast.s')], check=True)
subprocess.run(['ld', '-m', 'elf_i386', '-Ttext', '0x7c00', '--oformat', 'binary',
                '-o', str(out / 'boot.bin'), str(out / 'raycast.o')], check=True)
image = bytearray((out / 'boot.bin').read_bytes())
if len(image) >= 33 * 512:
    raise SystemExit('raycaster program exceeds reserved boot sectors')
image.extend(bytes(33 * 512 - len(image)))
for frame in frames:
    data = bytearray(40 * 512)
    vx, vy = frame['view_x'], frame['view_y']
    struct.pack_into('<4H2i4H', data, 0, 240, frame['angle'] * 10, vx >> 16, vy >> 16,
                     vx, vy, vx & 65535, (-vx) & 65535, vy & 65535, (-vy) & 65535)
    struct.pack_into('<H', data, 52, frame['pwall_pos'])
    data[0x1000:0x2000] = bytes(frame['tiles'])
    struct.pack_into('<240h', data, 0x3000, *tables['pixel_angles'])
    struct.pack_into('<64H', data, 0x3300, *frame['doors'])
    struct.pack_into('<900i', data, 0x4000, *tables['tangents'])
    image.extend(data)
(out / 'raycast.img').write_bytes(image)
visibility = out / 'visibility.bin'
run = subprocess.run([
    'qemu-system-i386', '-display', 'none', '-no-reboot', '-serial', 'none',
    '-monitor', 'none', '-debugcon', f'file:{visibility}',
    '-global', 'isa-debugcon.iobase=0xe9',
    '-device', 'isa-debug-exit,iobase=0xf4,iosize=0x04',
    '-drive', f'file={out / "raycast.img"},format=raw,if=ide',
    '-accel', 'tcg', '-m', '16M'], timeout=45)
# isa-debug-exit returns (0x10 << 1) | 1 after the final frame.
if run.returncode != 33:
    raise SystemExit(f'original raycaster VM failed: exit {run.returncode}')
masks = visibility.read_bytes()
if len(masks) != len(frames) * 4096:
    raise SystemExit(f'incomplete assembly output: {len(masks)} bytes')
for command, frame in enumerate(frames):
    actual = masks[command * 4096:(command + 1) * 4096]
    expected = bytes(frame['visible'])
    if actual != expected:
        differences = [{'x': n // 64, 'y': n % 64, 'original': actual[n], 'port': expected[n]}
                       for n in range(4096) if actual[n] != expected[n]]
        (out / 'result.json').write_text(json.dumps({'status': 'mismatch',
            'matched_commands': command, 'differences': differences}, indent=2) + '\n')
        raise SystemExit(f'command {command}: {len(differences)} floor visibility differences')
result = {'status': 'success', 'matched_commands': len(frames),
          'compared_floor_bits': len(masks), 'rays_per_command': 240}
(out / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
print(json.dumps(result))
