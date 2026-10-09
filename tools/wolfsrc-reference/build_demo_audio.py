"""Extract the original synthesized sound scheduler; hardware writes are no-ops."""
import pathlib
import re
import subprocess

from build_demo_player import function

SOURCE_HASHES = {
    'ID_SD.C': '8a7a64b36d9502958a2160ad15bfcc63b9743d3c632d7aa9795a5418173fa82c',
    'ID_SD_A.ASM': 'a023679dfc45fb68118b01044e18ea1f997e01ace81629f459646d1dd74de637',
    'AUDIOWL6.H': '44e25736a10adbed7d039915cdd3df8665c41639174f2ec5adb70fe98efa5f99',
}


def preprocess(source, cc, upload):
    return subprocess.run([cc, '-E', '-P', '-x', 'c', '-'] + (['-DUPLOAD'] if upload else []),
                          input=source, text=True, capture_output=True, check=True).stdout


def death_scream(source, cc):
    original = function(source, 'A_DeathScream')
    variants = []
    for suffix, upload in (('Shareware', True), ('Registered', False)):
        variants.append(preprocess(original, cc, upload).replace(
            'A_DeathScream', 'A_DeathScream' + suffix))
    return '\n'.join(variants) + '''
void A_DeathScream(objtype *ob) {
    if (reference_registered) A_DeathScreamRegistered(ob);
    else A_DeathScreamShareware(ob);
}
'''


def build_include(texts, directory: pathlib.Path, cc):
    header = texts['AUDIOWL6.H']
    sounds = re.search(r'typedef enum\s*\{.*?}\s*soundnames;', header, re.S).group()
    sounds = sounds.replace('typedef enum', 'enum').replace('} soundnames;', '};')
    sound_source = re.sub(r'\bword\s+(SD_SoundPlaying\s*\()', r'boolean \1', texts['ID_SD.C'])
    sound_source = re.sub(r'#ifdef\s+_MUSE_\s+void\s+#else\s+static void\s+#endif', 'void', sound_source)
    names = ('SDL_PCPlaySound', 'SDL_PCStopSound', 'SDL_PCService',
             'SDL_ALStopSound', 'SDL_ALPlaySound', 'SDL_ALSoundService',
             'SD_PlaySound', 'SD_SoundPlaying')
    routines = '\n\n'.join(function(sound_source, name) for name in names)
    # Interrupt masking and port I/O have no effect on the deterministic host
    # scheduler. Keep original sample/pointer counters and priority decisions.
    routines = re.sub(r'^\s*asm[^\n]*', '', routines, flags=re.M)
    routines = re.sub(r'\(long\)(pcSound|alSound)\s*=\s*0;', r'\1 = NULL;', routines)
    routines = routines.replace('soundnames sound', 'int sound')
    digi = re.search(r'static\s+int\s+wolfdigimap\[\]\s*=\s*\{.*?};', texts['WL_MAIN.C'], re.S).group()
    mappings = '\n'.join(preprocess(digi, cc, upload).replace('wolfdigimap', name)
                         for name, upload in (('shareware_digimap', True), ('registered_digimap', False)))
    # ID_SD_A.ASM's active DOFX decrements both counters once per 140Hz step.
    # The #if 0 C services extracted above are the matching original model;
    # only the hardware output differs. Pin the assembly as evidence as well.
    asm = texts['ID_SD_A.ASM']
    for instruction in ('dec\t[pcLengthLeft]', 'dec\t[alLengthLeft]'):
        if instruction not in asm:
            raise SystemExit('original DOFX sound counter instruction missing')
    (directory / 'original_demo_audio.inc').write_text(
        '/* Extracted from verified ID_SD.C and AUDIOWL6.H. */\n'
        + sounds + '\n' + mappings + '\n' + routines + '\n')
