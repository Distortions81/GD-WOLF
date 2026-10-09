/* Host adapters for verified original ID_SD.C scheduling. Digital routing is
   original, but digital sample duration/output is intentionally not modeled. */
#define huge
#define NUMSOUNDS 87
#define alFreqL 0xa0
#define alFreqH 0xb0
#define MK_FP(segment, offset) (segment)
#define SDL_SoundFinished() { SoundNumber = SoundPriority = 0; }
enum { sdm_Off, sdm_PC, sdm_AdLib };
enum { sds_Off, sds_PC, sds_SoundSource, sds_SoundBlaster };
typedef unsigned short word;
typedef struct { uint32_t length; word priority; } SoundCommon;
typedef struct { SoundCommon common; byte data[1]; } PCSound;
typedef struct {
    byte mChar, cChar, mScale, cScale, mAttack, cAttack, mSus, cSus;
    byte mWave, cWave, nConn, voice, mode, unused[3];
} Instrument;
typedef struct { SoundCommon common; Instrument inst; byte block, data[1]; } AdLibSound;
static SoundCommon *SoundTable[NUMSOUNDS];
static int SoundMode, DigiMode, SoundNumber, SoundPriority, DigiNumber, DigiPriority;
static int LeftPosition, RightPosition, nextsoundpos, SoundPositioned;
static int DigiMap[NUMSOUNDS], pcLastSample, alBlock;
static byte *pcSound, *alSound;
static unsigned long pcLengthLeft, alLengthLeft;
static word pcSoundLookup[256];
static Instrument alZeroInst;
static void alOut(int reg, int value) { (void)reg; (void)value; }
static void SDL_AlSetFXInst(Instrument *inst) { (void)inst; }
static void SD_PlayDigitized(int sound, int left, int right) { (void)sound; (void)left; (void)right; }
static void SDL_PCPlaySound(PCSound *sound);
static void SDL_PCStopSound(void);
static void SDL_ALStopSound(void);
static void SDL_ALPlaySound(AdLibSound *sound);
#include "original_demo_audio.inc"

static unsigned char *read_sound_file(const char *path, size_t *length) {
    if (!path || !*path) Quit("sound mode requires raw AUDIOHED/AUDIOT paths");
    FILE *file = fopen(path, "rb");
    if (!file || fseek(file, 0, SEEK_END)) Quit("cannot open raw audio bank");
    long size = ftell(file);
    if (size < 0 || size > 16*1024*1024 || fseek(file, 0, SEEK_SET)) Quit("invalid raw audio bank length");
    unsigned char *data = malloc((size_t)size + 1);
    if (!data || fread(data, 1, (size_t)size, file) != (size_t)size) Quit("cannot read raw audio bank");
    fclose(file); *length = (size_t)size; return data;
}
static uint32_t sound_u32(const byte *p) {
    return (uint32_t)p[0] | (uint32_t)p[1]<<8 | (uint32_t)p[2]<<16 | (uint32_t)p[3]<<24;
}
static void initialize_reference_profile(void) {
    const char *registered = getenv("GDWOLF_DEMO_REGISTERED");
    const char *map = getenv("GDWOLF_DEMO_MAP_INDEX");
    reference_registered = registered && !strcmp(registered, "1");
    mapon = map ? atoi(map) : 0;
}
static void initialize_reference_sound(void) {
    const char *mode = getenv("GDWOLF_DEMO_SOUND_MODE");
    if (!mode || !*mode || !strcmp(mode, "off")) SoundMode = sdm_Off;
    else if (!strcmp(mode, "adlib")) SoundMode = sdm_AdLib;
    else if (!strcmp(mode, "adlib-digi")) { SoundMode = sdm_AdLib; DigiMode = sds_SoundBlaster; }
    else if (!strcmp(mode, "pc")) SoundMode = sdm_PC;
    else Quit("unsupported deterministic sound mode");
    for (int i = 0; i < NUMSOUNDS; i++) DigiMap[i] = -1;
    const int *map_entry = reference_registered ? registered_digimap : shareware_digimap;
    for (; *map_entry != LASTSOUND; map_entry += 2) DigiMap[map_entry[0]] = map_entry[1];
    if (SoundMode == sdm_Off) return;
    size_t head_length, data_length;
    byte *head = read_sound_file(getenv("GDWOLF_DEMO_AUDIO_HEAD"), &head_length);
    byte *data = read_sound_file(getenv("GDWOLF_DEMO_AUDIO_DATA"), &data_length);
    for (int sound = 0; sound < NUMSOUNDS; sound++) {
        int chunk = sound + (SoundMode == sdm_PC ? 0 : NUMSOUNDS);
        if ((size_t)(chunk+2)*4 > head_length) Quit("audio bank offset table is truncated");
        uint32_t start = sound_u32(head+chunk*4), end = sound_u32(head+(chunk+1)*4);
        int prefix = SoundMode == sdm_PC ? 6 : 23;
        if (start > end || end > data_length || end-start < (unsigned)prefix) Quit("audio bank sound header is truncated");
        uint32_t length = sound_u32(data+start);
        if (length > 65535 || length > end-start-prefix) Quit("audio bank sound data is truncated or exceeds original counter");
        word priority = data[start+4] | (word)data[start+5]<<8;
        if (SoundMode == sdm_PC) {
            PCSound *effect = calloc(1, sizeof(*effect)+length);
            if (!effect) Quit("cannot allocate PC sound");
            effect->common.length = length; effect->common.priority = priority;
            memcpy(effect->data, data+start+prefix, length); SoundTable[sound] = &effect->common;
        } else {
            AdLibSound *effect = calloc(1, sizeof(*effect)+length);
            if (!effect) Quit("cannot allocate AdLib sound");
            effect->common.length = length; effect->common.priority = priority;
            memcpy(&effect->inst, data+start+6, sizeof(effect->inst));
            effect->block = data[start+22];
            memcpy(effect->data, data+start+prefix, length); SoundTable[sound] = &effect->common;
        }
    }
    free(head); free(data);
}
static void reference_sound_service(int steps) {
    for (int i = 0; i < steps; i++) { SDL_PCService(); SDL_ALSoundService(); }
}
static void SD_WaitSoundDone(void) {
    while (SD_SoundPlaying()) reference_sound_service(1);
}
static void print_sound_state(void) {
    unsigned long remaining = SoundMode == sdm_PC ? pcLengthLeft : alLengthLeft;
    if (!pcSound && !alSound) remaining = 0;
    printf("{\"playing\":%d,\"priority\":%d,\"remaining\":%lu}", SD_SoundPlaying(), SoundPriority, remaining);
}
static int sound_probe(void) {
    char action[32];
    while (scanf("%31s", action) == 1) {
        int amount;
        if (!strcmp(action, "reset")) {
            SDL_PCStopSound(); SDL_ALStopSound();
            SoundNumber = SoundPriority = DigiNumber = DigiPriority = 0;
            pcLengthLeft = alLengthLeft = 0;
            rng_index = facecount = gamestate.faceframe = 0;
        } else {
            if (scanf("%d", &amount) != 1) return 2;
            if (!strcmp(action, "play")) {
                if (amount < -1 || amount >= NUMSOUNDS) return 2;
                SD_PlaySound(amount);
            } else if (!strcmp(action, "advance") || !strcmp(action, "service")) {
                if (amount < 0 || amount > 10000) return 2;
                reference_sound_service(amount * (!strcmp(action, "advance") ? 2 : 1));
            } else if (!strcmp(action, "face")) {
                if (amount < 0 || amount > 70) return 2;
                tics = amount; OriginalUpdateFace();
            } else return 2;
        }
        printf("{\"sound\":"); print_sound_state();
        printf(",\"rng_index\":%d,\"face_timer\":%d,\"face_frame\":%d}\n", rng_index, facecount, gamestate.faceframe);
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
