/* Original demo runtime carries independent gameplay state. Floor visibility
   is supplied by the port and audited separately against original x86 assembly. */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#define far
#define UPLOAD
typedef int boolean;
typedef int32_t fixed;
typedef unsigned char byte;
typedef void *memptr;
enum { false, true };
enum { gd_baby, gd_easy, gd_medium, gd_hard };
enum { NORTH, EAST, SOUTH, WEST };
enum { wp_knife, wp_pistol, wp_machinegun, wp_chaingun };
enum { dr_normal, dr_lock1, dr_lock2, dr_lock3, dr_lock4, dr_elevator };
enum { bt_attack, bt_strafe, bt_run, bt_use, bt_readyknife };
enum { di_north, di_east, di_south, di_west };
#define TILESHIFT 16
#define UNSIGNEDSHIFT 8
#define TILEGLOBAL 65536
#define AREATILE 107
#define AMBUSHTILE 106
#define MAPSIZE 64
#define NUMENEMIES 22
#define SPDPATROL 512
#define SPDDOG 1500
#define MINACTORDIST 65536
#define MINSIGHT 0x18000
#define RUNSPEED 6000
#define ICONARROWS 90
#define FL_SHOOTABLE 1
#define FL_NEVERMARK 4
#define FL_VISABLE 8
#define FL_ATTACKMODE 16
#define FL_FIRSTATTACK 32
#define FL_AMBUSH 64
#define FL_NONMARK 128
#define PI 3.141592657
#define M_PI 3.14159265358979323846
#define LABS labs
#define PROJSIZE 0x2000
#define PROJECTILESIZE 0xc000L
#define BJRUNSPEED 2048
#define BJJUMPSPEED 680
#define MAXACTORS 150
#define FINEANGLES 3600
#define ANGLES 360
#define ANGLEQUAD 90
#define GLOBAL1 65536
#define VIEWGLOBAL 65536
#define MINDIST 0x5800
#define ACTORSIZE 0x4000
#define NUMAREAS 37
#define OPENTICS 300
#define MAXSTATS 400
#define EXTRAPOINTS 40000
#define FL_BONUS 2
#define GOTGATLINGPIC 0
#define ex_completed 1
#define PLAYERSIZE MINDIST
#define MOVESCALE 150
#define BACKMOVESCALE 100
#define ANGLESCALE 20
#define EXITTILE 99
#define PUSHABLETILE 98
#define ELEVATORTILE 21
#define ALTELEVATORTILE 107
#define ex_secretlevel 9
#define ex_died 2
#define ex_victorious 6
#define MAPSPOT(x,y,p) (mapsegs[p][(y)*64+(x)])
#include "original_demo_runtime_types.inc"
typedef struct object objtype;
typedef struct statestruct {
    int rotate, shapenum, tictime;
    void (*think)(objtype *), (*action)(objtype *);
    struct statestruct *next;
} statetype;
struct object {
    statetype *state;
    int ticcount, tilex, tiley, flags, obclass, hitpoints, active, temp2;
    byte areanumber;
    dirtype dir;
    long x, y, speed, distance;
    long transx;
    int viewx;
    int viewheight;
    int angle;
    int temp1;
    fixed transy;
    objtype *next, *prev;
};
typedef struct { int tilex, tiley, vertical, lock, action, ticcount; } doorobj_t;
static struct { int difficulty, killtotal, secrettotal, victoryflag, bestweapon, weapon, killcount, keys, ammo, chosenweapon, attackcount, attackframe, weaponframe, secretcount, health, treasuretotal, treasurecount, lives, faceframe, score, nextextra; long killx, killy, TimeCount; } gamestate;
static objtype objects[MAXACTORS], *new, *actorat[64][64], player_storage, *player = &player_storage;
#define objlist objects
static objtype *objfreelist, *lastobj;
static int objcount, mapwidth, mapheight, loadedgame, rng_index, tics, madenoise;
static int reference_registered, mapon;
static int compact_probe;
static int reference_command_index = -1;
static unsigned short planes[2][4096], *mapsegs[] = {planes[0], planes[1]};
static unsigned farmapylookup[64], plux, pluy;
static unsigned char tilemap[64][64];
static doorobj_t doorobjlist[64], *lastdoorobj = doorobjlist;
static int doornum, areabyplayer[NUMAREAS], doorposition[64], thrustspeed, player_damage;
static unsigned char areaconnect[NUMAREAS][NUMAREAS];
static unsigned pwallstate, pwallpos, pwallx, pwally;
static int pwalldir;
static byte spotvis[64][64];
typedef struct { int shapenum, tilex, tiley, flags, itemnumber; byte *visspot; } statobj_t;
static statobj_t statobjlist[MAXSTATS], *laststatobj = statobjlist;
static int facecount, gotgatgun, spearflag, spearangle, playstate;
static long spearx, speary;
static int godmode;
static objtype *LastAttacker, *killerobj;
#define MAXVISABLE 50
static struct { int shapenum, viewx, viewheight; } vislist[MAXVISABLE], *visptr;
static void TransformActor(objtype *ob);
static int CalcRotate(objtype *ob);
static long tileglobal = TILEGLOBAL;
static char str[256];
static int centerx = 119, shootdelta = 24;
static int viewwidth = 240, viewheight = 120, pixelangle[240], minheightdiv;
static fixed finetangent[900], sintable[451], focallength, scale, maxslope;
static fixed *costable = sintable+90;
static fixed viewx, viewy, viewcos, viewsin, mindist = MINDIST;
static long heightnumerator;
static const float radtoint = (float)FINEANGLES/2/PI;
static int buttonstate[8], buttonheld[8], reference_shots;
static int controlx, controly, anglefrac, noclip;
static long playerxmove, playerymove;
static void Thrust(int angle, long speed);
static void ClipMove(objtype *ob, long xmove, long ymove);
static void OriginalControlMovement(objtype *ob);
static void OriginalUpdateFace(void);
static void BuildTables(void);
static fixed FixedByFrac(fixed value, fixed fraction) {
    int64_t magnitude = value < 0 ? -(int64_t)value : value;
    int64_t result = magnitude * ((uint32_t)fraction & 65535) >> 16;
    return ((value < 0) != (fraction < 0)) ? -result : result;
}
static int US_RndT(void);
static void SpawnStatic(int x, int y, int type);
static boolean TryWalk(objtype *ob);
static void Quit(const char *message) { fprintf(stderr, "%s\n", message); exit(2); }
/* Only this executable's near-data aliases have independent DOS evidence.
   No host-C adjacency is used to define out-of-bounds source behavior. */
#define REFERENCE_DOS_PROFILE "registered-apogee-v1.4-2a969a97"
static int reference_dos_profile;
static doorobj_t reference_alias_door;
#define UPDATEWIDE 20
#define UPDATEHIGH 13
#define PIXTOBLOCK 4
static byte update[UPDATEHIGH][UPDATEWIDE], *updateptr = &update[0][0];
static int uwidthtable[UPDATEHIGH];
static int VW_MarkUpdateBlock(int x1, int y1, int x2, int y2);
static int reference_validate_area_index(const char *routine, int index);
static void initialize_reference_memory(void) {
    const char *profile = getenv("GDWOLF_DEMO_MEMORY_PROFILE");
    if (profile && *profile && strcmp(profile, "strict-source")) {
        if (strcmp(profile, REFERENCE_DOS_PROFILE)) Quit("unknown original DOS memory profile");
        reference_dos_profile = 1;
    }
    for (int i = 0; i < UPDATEHIGH; i++) uwidthtable[i] = i*UPDATEWIDE;
}
static void reference_validate_door_index(const char *routine, int index) {
    if (index >= 0 && index < 64) return;
    if (reference_dos_profile && index >= 64 && index < 128 &&
        (!strcmp(routine, "OperateDoor") || !strcmp(routine, "OpenDoor") || !strcmp(routine, "CloseDoor"))) return;
    fprintf(stderr, "unsupported original memory access: %s door=%d outside doorobjlist[64]; command=%d player=(%d,%d) angle=%d\n",
        routine, index, reference_command_index, player->tilex, player->tiley, player->angle);
    exit(3);
}
static int reference_area_by_player(const char *routine, int index) {
    if (index >= 0 && index < NUMAREAS) return areabyplayer[index];
    if (reference_dos_profile && index == 255)
        return updateptr[36] | (updateptr[37] << 8);
    return areabyplayer[reference_validate_area_index(routine, index)];
}
static byte reference_dos_static_byte(int offset) {
    if (offset == -2 || offset == -1) {
        unsigned ptr = 0x803a + (laststatobj-statobjlist)*8;
        return (byte)(ptr >> ((offset+2)*8));
    }
    if (offset < 0 || offset >= MAXSTATS*8) Quit("DOS static alias outside verified region");
    if (offset >= (laststatobj-statobjlist)*8) {
        fprintf(stderr, "unsupported original memory access: DOS static alias references an unallocated slot; command=%d offset=%d\n", reference_command_index, offset);
        exit(3);
    }
    statobj_t *obj = &statobjlist[offset/8];
    unsigned field;
    switch (offset%8) {
        case 0: return (byte)obj->tilex;
        case 1: return (byte)obj->tiley;
        case 2: case 3:
            field = obj->visspot ? 0x9c48 + (obj->visspot-&spotvis[0][0]) : 0;
            return (byte)(field >> ((offset%8-2)*8));
        case 4: case 5: return (byte)((unsigned)obj->shapenum >> ((offset%8-4)*8));
        case 6: return (byte)obj->flags;
        default: return (byte)obj->itemnumber;
    }
}
static doorobj_t *reference_door(const char *routine, int index) {
    if (index >= 0 && index < 64) return &doorobjlist[index];
    reference_validate_door_index(routine, index);
    byte raw[10];
    for (int i = 0; i < 10; i++) raw[i] = reference_dos_static_byte(index*10-642+i);
    reference_alias_door.tilex = raw[0]; reference_alias_door.tiley = raw[1];
    reference_alias_door.vertical = raw[2] | raw[3]<<8;
    reference_alias_door.lock = raw[4]; reference_alias_door.action = (int16_t)(raw[6] | raw[7]<<8);
    reference_alias_door.ticcount = (int16_t)(raw[8] | raw[9]<<8);
    return &reference_alias_door;
}
static void reference_dos_static_word(int offset, unsigned value) {
    if (offset < 0 || offset+1 >= (laststatobj-statobjlist)*8 || (offset&1)) {
        fprintf(stderr, "unsupported original memory access: DOS static alias write outside allocated slots; command=%d offset=%d\n", reference_command_index, offset);
        exit(3);
    }
    statobj_t *obj = &statobjlist[offset/8];
    value &= 65535;
    switch (offset%8) {
        case 0: obj->tilex = value&255; obj->tiley = value>>8; break;
        case 2:
            if (value < 0x9c48 || value >= 0xac48) {
                fprintf(stderr, "unsupported original memory access: DOS static visibility pointer outside spotvis; command=%d pointer=%u\n", reference_command_index, value);
                exit(3);
            }
            obj->visspot = &spotvis[0][0]+value-0x9c48;
            break;
        case 4: obj->shapenum = (int16_t)value; break;
        case 6: obj->flags = value&255; obj->itemnumber = value>>8; break;
    }
}
static void reference_commit_door(int index) {
    if (index < 64) return;
    int offset = index*10-642;
    unsigned action = (uint16_t)reference_alias_door.action;
    unsigned timer = (uint16_t)reference_alias_door.ticcount;
    if ((reference_dos_static_byte(offset+6) | reference_dos_static_byte(offset+7)<<8) != action)
        reference_dos_static_word(offset+6, action);
    if ((reference_dos_static_byte(offset+8) | reference_dos_static_byte(offset+9)<<8) != timer)
        reference_dos_static_word(offset+8, timer);
}
static objtype *reference_actor_at(const char *routine, int x, int y) {
    if (x < 0 || x >= 64 || y < 0 || y >= 64) {
        fprintf(stderr, "unsupported original memory access: %s actorat[%d][%d] outside map; command=%d\n", routine, x, y, reference_command_index);
        exit(3);
    }
    return actorat[x][y];
}
static long reference_object_coordinate(const char *routine, objtype *ob, int y) {
    if ((uintptr_t)ob < (uintptr_t)objects || (uintptr_t)ob >= (uintptr_t)(objects+MAXACTORS)) {
        fprintf(stderr, "unsupported original memory access: %s dereferenced non-object near pointer=%lu; command=%d\n", routine, (unsigned long)(uintptr_t)ob, reference_command_index);
        exit(3);
    }
    return y ? ob->y : ob->x;
}
static int reference_validate_area_index(const char *routine, int index) {
    if (index >= 0 && index < NUMAREAS) return index;
    fprintf(stderr, "unsupported original memory access: %s area=%d outside original areas[%d]; command=%d player=(%d,%d) angle=%d\n",
        routine, index, NUMAREAS, reference_command_index, player->tilex, player->tiley, player->angle);
    exit(3);
}
static int reference_validate_door_position(const char *routine, int index) {
    if (index >= 0 && index < 64) return index;
    fprintf(stderr, "unsupported original memory access: %s doorposition[%d] outside original positions[64]; command=%d\n", routine, index, reference_command_index);
    exit(3);
}
static void GetNewActor(void);
static void OpenDoor(int door);
static void ConnectAreas(void);
static void TakeDamage(int points, objtype *attacker);
static void StartDamageFlash(int points) { (void)points; }
static boolean SD_PlaySound(int sound);
static int SD_SoundPlaying(void);
static void SD_WaitSoundDone(void);
static void PlaySoundLocActor(int sound, objtype *ob) { (void)ob; SD_PlaySound(sound); }
static void PlaySoundLocTile(int sound, int x, int y) { (void)x; (void)y; SD_PlaySound(sound); }
static void DrawWeapon(void) {}
static void DrawAmmo(void) {}
static void DrawHealth(void) {}
static void DrawFace(void) {}
static void DrawKeys(void) {}
static void DrawLives(void) {}
static void DrawScore(void) {}
static void StartBonusFlash(void) {}
static void StatusDrawPic(int x, int y, int pic) { (void)x; (void)y; (void)pic; }
static void UpdateFace(void) { OriginalUpdateFace(); }
static void ControlMovement(objtype *ob) { OriginalControlMovement(ob); }
static void PlaceItemType(int type, int x, int y);
static void GivePoints(long points);
/* Death-camera presentation does not change its gameplay geometry or state
   transitions. Waiting, pixels and user acknowledgement remain outside the
   deterministic command-clock reference. The original routine executes. */
#define STATUSLINES 40
#define LEVELEND_LUMP_START 0
#define LEVELEND_LUMP_END 0
#define STR_SEEAGAIN ""
static int bufferofs, displayofs, screenloc[3], fizzlein;
static void FinishPaletteShifts(void) {}
static void VW_WaitVBL(int count) { (void)count; }
static void VW_Bar(int x, int y, int width, int height, int color) { (void)x; (void)y; (void)width; (void)height; (void)color; }
static void FizzleFade(int source, int dest, int width, int height, int frames, int abortable) { (void)source; (void)dest; (void)width; (void)height; (void)frames; (void)abortable; }
static void PM_UnlockMainMem(void) {}
static void CA_UpLevel(void) {}
static void CacheLump(int start, int end) { (void)start; (void)end; }
static void Write(int x, int y, const char *text) { (void)x; (void)y; (void)text; }
static void CA_DownLevel(void) {}
static void PM_CheckMainMem(void) {}
static void VW_UpdateScreen(void) {
    /* ID_VH_A.ASM VH_UpdateScreen: clear only dirty bytes whose low bit is set. */
    for (int i = 0; i < UPDATEWIDE*UPDATEHIGH; i++) if (updateptr[i]&1) updateptr[i] = 0;
}
static void IN_UserInput(int delay) { (void)delay; }
static void DrawPlayBorder(void);
static void VWB_Bar(int x, int y, int width, int height, int color) { (void)color; VW_MarkUpdateBlock(x,y,x+width,y+height-1); }
static void VWB_Hlin(int x1, int x2, int y, int color) { (void)color; VW_MarkUpdateBlock(x1,y,x2,y); }
static void VWB_Vlin(int y1, int y2, int x, int color) { (void)color; VW_MarkUpdateBlock(x,y1,x,y2); }
static void VWB_Plot(int x, int y, int color) { (void)color; VW_MarkUpdateBlock(x,y,x,y); }
#include "demo_audio_support.inc"
#include "original_demo_runtime.inc"
static int US_RndT(void) { rng_index = (rng_index + 1) & 255; return rndtable[rng_index]; }
static int kind(objtype *ob) {
    switch (ob->obclass) {
        case guardobj: return 0; case officerobj: return 1; case ssobj: return 2;
        case dogobj: return 3; case bossobj: return 4; case mutantobj: return 5; case gretelobj: return 6;
        case schabbobj: return 7; case giftobj: return 8; case fatobj: return 9; case fakeobj: return 10;
        case mechahitlerobj: return 11; case realhitlerobj: return 12; case ghostobj: return 13;
        case needleobj: return 14; case rocketobj: return 15; case fireobj: return 16;
        case inertobj:
            if (ob->state == &s_smoke1 || ob->state == &s_smoke2 || ob->state == &s_smoke3 || ob->state == &s_smoke4) return 17;
            return -1;
        default: return -1;
    }
}
static void print_stats(void) {
    printf("{\"health\":%d,\"ammo\":%d,\"lives\":%d,\"keys\":%d,\"score\":%d,\"next_extra\":%d,\"weapon\":%d,\"best_weapon\":%d,\"chosen_weapon\":%d,\"secrets\":%d,\"secret_total\":%d,\"treasure\":%d,\"treasure_total\":%d,\"kills\":%d,\"kill_total\":%d,\"time_count\":%ld}",
        gamestate.health, gamestate.ammo, gamestate.lives, gamestate.keys, gamestate.score, gamestate.nextextra,
        gamestate.weapon, gamestate.bestweapon, gamestate.chosenweapon, gamestate.secretcount, gamestate.secrettotal,
        gamestate.treasurecount, gamestate.treasuretotal, gamestate.killcount, gamestate.killtotal, gamestate.TimeCount);
}
static void print_victory(void) {
    printf("{\"active\":%s,\"actors\":[", gamestate.victoryflag ? "true" : "false");
    int printed = 0;
    for (objtype *bj = player->next; bj; bj = bj->next) if (bj->obclass == bjobj) {
        printf("%s{\"x\":%ld,\"y\":%ld,\"tile_x\":%d,\"tile_y\":%d,\"dir\":%d,\"distance\":%ld,\"remaining_tiles\":%d,\"shape\":%d,\"tic_count\":%d,\"frame_tics\":%d,\"state\":%d,\"actor_active\":%s}",
            printed++ ? "," : "", bj->x, bj->y, bj->tilex, bj->tiley, bj->dir, bj->distance, bj->temp1,
            bj->state->shapenum, bj->ticcount, bj->state->tictime, reference_actor_state(bj->state), bj->active ? "true" : "false");
    }
    printf("]}");
}
static void print_static_slots(void) {
    const char *enabled = getenv("GDWOLF_DEMO_STATICS");
    if (!enabled || strcmp(enabled, "1")) return;
    printf(",\"statics\":[");
    for (statobj_t *spot = statobjlist; spot < laststatobj; spot++)
        printf("%s{\"x\":%d,\"y\":%d,\"shape\":%d,\"flags\":%d,\"item\":%d,\"visibility\":%ld}",
            spot == statobjlist ? "" : ",", spot->tilex, spot->tiley, spot->shapenum, spot->flags, spot->itemnumber, (long)(spot->visspot-&spotvis[0][0]));
    printf("]");
}
static void print_state(void) {
    printf("{\"rng_index\":%d,\"damage\":%d,\"actors\":[", rng_index, player_damage);
    int printed = 0;
    for (objtype *ob = player->next; ob; ob = ob->next) {
        if (kind(ob) < 0) continue;
        printf("%s{\"kind\":%d,\"x\":%ld,\"y\":%ld,\"tile_x\":%d,\"tile_y\":%d,\"dir\":%d,"
               "\"area\":%d,\"distance\":%ld,\"reaction\":%d,\"health\":%d,\"flags\":%d,\"shape\":%d,\"tic_count\":%d,\"active\":%s,\"state\":%d,\"frame_tics\":%d,\"angle\":%d,\"speed\":%ld,\"pool_slot\":%d}",
               printed++ ? "," : "", kind(ob), ob->x, ob->y, ob->tilex, ob->tiley, ob->dir,
               ob->areanumber, ob->distance, ob->temp2, ob->hitpoints > 0 ? ob->hitpoints : 0,
               ob->flags & (FL_SHOOTABLE|FL_ATTACKMODE|FL_FIRSTATTACK|FL_AMBUSH|FL_NONMARK|FL_NEVERMARK), ob->state->shapenum, ob->ticcount,
               ob->active ? "true" : "false", reference_actor_state(ob->state), ob->state->tictime, ob->angle, ob->speed, (int)(ob-objects));
    }
    printf("],\"doors\":[");
    for (int i = 0; i < doornum; i++) printf("%s{\"action\":%d,\"position\":%d,\"timer\":%d}", i ? "," : "", doorobjlist[i].action, doorposition[i], doorobjlist[i].action == dr_open ? doorobjlist[i].ticcount : 0);
    printf("],\"walls\":[");
    if (!compact_probe) for (int y = 0; y < 64; y++) for (int x = 0; x < 64; x++) printf("%s%d", x || y ? "," : "", tilemap[x][y] && !(tilemap[x][y] >= 128 && tilemap[x][y] < 192));
    printf("],\"weapon\":{\"type\":%d,\"attacking\":%s,\"frame\":%d,\"timer\":%d,\"ammo\":%d,\"shots\":%d},\"player\":{\"x\":%ld,\"y\":%ld,\"angle\":%d,\"angle_frac\":%d},\"stats\":", gamestate.weapon, player->state == &s_attack ? "true" : "false", gamestate.attackframe, player->state == &s_attack ? gamestate.attackcount : 0, gamestate.ammo, reference_shots, player->x, player->y, player->angle, anglefrac);
    print_stats();
    printf(",\"face\":{\"frame\":%d,\"timer\":%d},\"pushwall\":{\"active\":%s,\"x\":%u,\"y\":%u,\"dir\":%d,\"state\":%u,\"position\":%u},\"areas\":[", gamestate.faceframe, facecount, pwallstate ? "true" : "false", pwallstate ? pwallx : 0, pwallstate ? pwally : 0, pwallstate ? pwalldir : 0, pwallstate, pwallstate ? pwallpos : 0);
    for (int i = 0; i < 64; i++) printf("%s%s", i ? "," : "", i < NUMAREAS && areabyplayer[i] ? "true" : "false");
    printf("],\"sound\":"); print_sound_state();
    printf(",\"victory\":"); print_victory();
    printf(",\"player_state\":%d,\"terminal\":%d,\"bonuses\":[", player->state == &s_deathcam ? 2 : player->state == &s_attack ? 1 : 0, playstate);
    printed = 0;
    for (statobj_t *spot = statobjlist; spot < laststatobj; spot++) if (spot->shapenum >= 0 && (spot->flags & FL_BONUS))
        printf("%s{\"x\":%d,\"y\":%d,\"shape\":%d}", printed++ ? "," : "", spot->tilex, spot->tiley, spot->shapenum);
    printf("]");
    const char *occupancy = getenv("GDWOLF_DEMO_OCCUPANCY");
    if (occupancy && !strcmp(occupancy, "1")) {
        printf(",\"actorat\":[");
        for (int y = 0; y < 64; y++) for (int x = 0; x < 64; x++) {
            objtype *ob = actorat[x][y];
            int tag = (uintptr_t)ob < 256 ? (int)(uintptr_t)ob : 256+(int)(ob-objects);
            printf("%s%d", x || y ? "," : "", tag);
        }
        printf("]");
    }
    const char *area_plane = getenv("GDWOLF_DEMO_AREA_PLANE");
    if (area_plane && !strcmp(area_plane, "1")) {
        printf(",\"area_plane\":[");
        for (int i = 0; i < 4096; i++) printf("%s%u", i ? "," : "", mapsegs[0][i]);
        printf("]");
    }
    print_static_slots();
    printf(",\"memory_profile\":\"%s\",\"area_255_word\":%d", reference_dos_profile ? REFERENCE_DOS_PROFILE : "strict-source", updateptr[36] | updateptr[37]<<8);
    puts("}");
    fflush(stdout);
}
#include "demo_static_probe.inc"
/* Exact raycaster inputs, separate from the gameplay comparison stream. */
static void record_raycast(void) {
    static FILE *trace;
    static int initialized;
    if (!initialized) {
        const char *path = getenv("GDWOLF_DEMO_RAYCAST_OUT");
        if (path && *path) {
            trace = fopen(path, "w");
            if (!trace) Quit("cannot open raycaster trace");
        }
        initialized = 1;
    }
    if (!trace) return;
    fprintf(trace, "{\"view_x\":%d,\"view_y\":%d,\"angle\":%d,\"pwall_pos\":%u,\"tiles\":[", viewx, viewy, player->angle, pwallpos);
    for (int x = 0; x < 64; x++) for (int y = 0; y < 64; y++) fprintf(trace, "%s%d", x || y ? "," : "", tilemap[x][y]);
    fprintf(trace, "],\"doors\":[");
    for (int i = 0; i < 64; i++) fprintf(trace, "%s%d", i ? "," : "", doorposition[i]);
    fprintf(trace, "],\"visible\":[");
    for (int x = 0; x < 64; x++) for (int y = 0; y < 64; y++) fprintf(trace, "%s%d", x || y ? "," : "", spotvis[x][y]);
    fprintf(trace, "]}\n");
    fflush(trace);
}
/* Probe original reward routines at boundary values the recorded routes may
   never reach. Each line resets the supplied inventory before one action. */
static int bonus_probe(void) {
    const struct { const char *name; int item; } items[] = {
        {"cross", bo_cross}, {"chalice", bo_chalice}, {"bible", bo_bible}, {"crown", bo_crown},
        {"fullheal", bo_fullheal}, {"food", bo_food}, {"medkit", bo_firstaid}, {"alpo", bo_alpo},
        {"gibs", bo_gibs}, {"ammo", bo_clip}, {"ammo2", bo_clip2}, {"key1", bo_key1},
        {"key2", bo_key2}, {"key3", bo_key3}, {"key4", bo_key4},
        {"machinegun", bo_machinegun}, {"chaingun", bo_chaingun}
    };
    char action[32];
    while (scanf("%31s", action) == 1) {
        memset(&gamestate, 0, sizeof(gamestate));
        int amount;
        if (scanf("%d %d %d %d %d %d %d %d %d %d %d", &gamestate.health, &gamestate.ammo,
            &gamestate.score, &gamestate.nextextra, &gamestate.lives, &gamestate.bestweapon,
            &gamestate.chosenweapon, &gamestate.weapon, &gamestate.keys, &gamestate.attackframe, &amount) != 11) return 2;
        if (gamestate.health < 0 || gamestate.health > 100 || gamestate.ammo < 0 || gamestate.ammo > 99 ||
            gamestate.score < 0 || gamestate.score > 1000000 || gamestate.nextextra < 1 ||
            gamestate.nextextra > 1040000 || gamestate.lives < 0 || gamestate.lives > 9 ||
            gamestate.bestweapon < 0 || gamestate.bestweapon > 3 || gamestate.chosenweapon < 0 ||
            gamestate.chosenweapon > 3 || gamestate.weapon < 0 || gamestate.weapon > 3 ||
            gamestate.keys < 0 || gamestate.keys > 15 || gamestate.attackframe < 0 || gamestate.attackframe > 13 ||
            amount < 0 || amount > 1000000) return 2;
        facecount = 23; gotgatgun = 0;
        int collected = 0;
        if (!strcmp(action, "points")) GivePoints(amount);
        else {
            int item = -1;
            for (unsigned i = 0; i < sizeof(items)/sizeof(items[0]); i++)
                if (!strcmp(action, items[i].name)) item = items[i].item;
            if (item < 0) return 2;
            statobj_t bonus = {0};
            bonus.itemnumber = item;
            GetBonus(&bonus);
            collected = bonus.shapenum == -1;
        }
        printf("{\"stats\":"); print_stats();
        printf(",\"weapon\":%d,\"face_timer\":%d,\"collected\":%s}\n", gamestate.weapon, facecount, collected ? "true" : "false");
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
/* Exercise original elevator orientation, use-edge and exit-kind decisions. */
static int elevator_probe(void) {
    int angle, floor, held;
    while (scanf("%d %d %d", &angle, &floor, &held) == 3) {
        if (angle < 0 || angle >= 360 || (floor != 107 && floor != 108) || held < 0 || held > 1) return 2;
        memset(planes, 0, sizeof(planes));
        memset(tilemap, 0, sizeof(tilemap));
        memset(buttonheld, 0, sizeof(buttonheld));
        for (int y = 0; y < 64; y++) farmapylookup[y] = y*64;
        player->x = player->y = 4*65536L+32768;
        player->tilex = player->tiley = 4;
        player->angle = angle;
        planes[0][4*64+4] = floor;
        tilemap[4][3] = tilemap[5][4] = tilemap[4][5] = tilemap[3][4] = ELEVATORTILE;
        buttonheld[bt_use] = held;
        playstate = 0;
        Cmd_Use();
        printf("{\"terminal\":%d,\"north_wall\":%d,\"east_wall\":%d,\"south_wall\":%d,\"west_wall\":%d,\"held\":%s}\n",
            playstate, tilemap[4][3], tilemap[5][4], tilemap[4][5], tilemap[3][4], buttonheld[bt_use] ? "true" : "false");
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
/* Independent MoveObj contact and inclusive-distance boundary checks. */
static int moveobj_probe(void) {
    long px, py, x, y, speed;
    int dir, connected, steps, actor_kind;
    while (scanf("%ld %ld %ld %ld %d %d %ld %d %d", &px, &py, &x, &y, &dir, &connected, &speed, &steps, &actor_kind) == 9) {
        if (dir < 0 || dir > 8 || connected < 0 || connected > 1 || speed < 0 || speed > 65536 || steps < 1 || steps > 70 || (actor_kind != 0 && actor_kind != 13)) return 2;
        objtype ob = {0};
        ob.x = x; ob.y = y; ob.dir = dir; ob.areanumber = 1; ob.distance = 65536;
        ob.obclass = actor_kind == 13 ? ghostobj : guardobj;
        player->x = px; player->y = py;
        gamestate.health = 100; gamestate.difficulty = gd_hard;
        player_damage = 0; tics = steps;
        areabyplayer[1] = connected;
        MoveObj(&ob, speed*steps);
        printf("{\"x\":%ld,\"y\":%ld,\"distance\":%ld,\"health\":%d,\"damage\":%d}\n", ob.x, ob.y, ob.distance, gamestate.health, player_damage);
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
int main(int argc, char **argv) {
    initialize_reference_profile();
    initialize_reference_memory();
    if (argc == 2 && !strcmp(argv[1], "--sound-probe")) { initialize_reference_sound(); return sound_probe(); }
    if (argc == 2 && !strcmp(argv[1], "--bonus-probe")) return bonus_probe();
    if (argc == 2 && !strcmp(argv[1], "--elevator-probe")) return elevator_probe();
    if (argc == 2 && !strcmp(argv[1], "--moveobj-probe")) return moveobj_probe();
    if (argc == 2 && !strcmp(argv[1], "--static-probe")) return static_probe();
    if (argc == 2 && !strcmp(argv[1], "--render-tables")) {
        BuildTables(); CalcProjection(0x5700);
        printf("{\"tangents\":[");
        for (int i = 0; i < 900; i++) printf("%s%d", i ? "," : "", finetangent[i]);
        printf("],\"pixel_angles\":[");
        for (int i = 0; i < viewwidth; i++) printf("%s%d", i ? "," : "", pixelangle[i]);
        puts("]}");
        return 0;
    }
    if (argc == 2 && !strcmp(argv[1], "--projection")) {
        int angle, x, y, grabbed, screen, height;
        long px, py;
        BuildTables(); CalcProjection(0x5700);
        while (scanf("%ld %ld %d %d %d", &px, &py, &angle, &x, &y) == 5) {
            if (angle < 0 || angle >= 360) return 2;
            viewcos = sintable[angle+90]; viewsin = sintable[angle];
            viewx = px - FixedByFrac(0x5700, viewcos);
            viewy = py + FixedByFrac(0x5700, viewsin);
            objtype ob = {0}; ob.x = x; ob.y = y;
            TransformActor(&ob);
            grabbed = TransformTile(x >> 16, y >> 16, &screen, &height);
            printf("{\"view_x\":%d,\"trans_x\":%ld,\"height\":%d,\"pickup\":%s}\n", ob.viewx, ob.transx, ob.viewheight, grabbed ? "true" : "false");
            fflush(stdout);
        }
        return ferror(stdin) || ferror(stdout) ? 2 : 0;
    }
    int damage_probe = argc == 2 && !strcmp(argv[1], "--enemy-damage-probe");
    int enemy_probe = damage_probe || (argc == 2 && !strcmp(argv[1], "--enemy-probe"));
    int victory_probe = argc == 2 && !strcmp(argv[1], "--victory-probe");
    if (argc != 1 && !enemy_probe && !victory_probe) return 2;
    compact_probe = victory_probe;
    if (!enemy_probe || getenv("GDWOLF_DEMO_AUDIO_HEAD")) initialize_reference_sound();
    if (scanf("%d %d %d", &mapwidth, &mapheight, &gamestate.difficulty) != 3 || mapwidth != 64 || mapheight != 64 || gamestate.difficulty != 3) return 2;
    for (int p = 0; p < 2; p++) for (int i = 0; i < 4096; i++) {
        unsigned value;
        if (scanf("%u", &value) != 1 || value > 65535) return 2;
        planes[p][i] = value;
    }
    for (int y = 0; y < 64; y++) {
        farmapylookup[y] = y*64;
        for (int x = 0; x < 64; x++) {
            int tile = planes[0][y*64+x];
            if ((x == 0 || y == 0 || x == 63 || y == 63) && planes[1][y*64+x] >= 108) return 2;
            if (tile < AREATILE) { tilemap[x][y] = tile; actorat[x][y] = (objtype *)(uintptr_t)tile; }
        }
    }
    for (int y = 1; y < 63; y++) for (int x = 1; x < 63; x++) {
        int tile = planes[0][y*64+x];
        if (tile >= 90 && tile <= 101) SpawnDoor(x, y, !(tile & 1), (tile-90)/2);
    }
    InitActorList();
    ScanInfoPlane();
    BuildTables();
    player->state = &s_player; gamestate.ammo = 8; gamestate.health = 100;
    gamestate.lives = 3; gamestate.nextextra = EXTRAPOINTS;
    InitAreas();
    ClearAmbushMarkers();
    gamestate.weapon = gamestate.bestweapon = gamestate.chosenweapon = 1;
    /* DrawPlayScreen marks the play border and the full status bar before
       PlayLoop begins; DOS captures verify these bytes at TimeCount zero. */
    DrawPlayBorder();
    VW_MarkUpdateBlock(0,160,319,199);
    if (victory_probe) {
        int angle, attacking, trigger;
        if (scanf("%d %d %d", &angle, &attacking, &trigger) != 3 || angle < 0 || angle >= 360 || attacking < 0 || attacking > 1 || trigger < 0 || trigger > 1) return 2;
        player->angle = angle;
        if (attacking) Cmd_Fire();
        if (trigger) VictoryTile();
    }
    print_state();
    if (victory_probe) {
        int buttons, raw_x, raw_y;
        while (scanf("%d %d %d", &buttons, &raw_x, &raw_y) == 3) {
            if (buttons < 0 || buttons > 255 || raw_x < -128 || raw_x > 127 || raw_y < -128 || raw_y > 127) return 2;
            reference_command_index++;
            for (int i = 0; i < 8; i++) { buttonheld[i] = buttonstate[i]; buttonstate[i] = (buttons >> i) & 1; }
            tics = 4; controlx = raw_x*tics; controly = raw_y*tics;
            reference_shots = 0; madenoise = 0; player_damage = 0;
            reference_sound_service(tics*2);
            MoveDoors(); MovePWalls();
            for (objtype *ob = player; ob; ob = ob->next) DoActor(ob);
            /* This probe supplies a deliberately complete visibility mask.
               It tests victory gameplay, not the raycaster's floor marking. */
            memset(spotvis, 1, sizeof(spotvis));
            CalcProjection(0x5700);
            viewcos = sintable[player->angle+90]; viewsin = sintable[player->angle];
            viewx = player->x - FixedByFrac(0x5700, viewcos);
            viewy = player->y + FixedByFrac(0x5700, viewsin);
            RefreshActorVisibility();
            reference_finish_tic();
            print_state();
            if (playstate) break;
        }
        return ferror(stdin) || ferror(stdout) ? 2 : 0;
    }
    if (enemy_probe) {
        int target, ordinal = 0, found = 0;
        if (scanf("%d", &target) != 1 || target < 0) return 2;
        for (objtype *ob = player->next; ob; ob = ob->next) if (kind(ob) >= 0) {
            if (ordinal++ == target) { ob->active = true; found = 1; break; }
        }
        if (!found) return 2;
        long probe_x, probe_y;
        int probe_noise, probe_tics;
        while (scanf("%ld %ld %d %d", &probe_x, &probe_y, &probe_noise, &probe_tics) == 4) {
            if (probe_x < 65536 || probe_x >= 63L*65536 || probe_y < 65536 || probe_y >= 63L*65536 ||
                (probe_noise != 0 && probe_noise != 1) || probe_tics < 1 || probe_tics > 70) return 2;
            reference_command_index++;
            int damage_target = -1, damage = 0;
            if (damage_probe && (scanf("%d %d", &damage_target, &damage) != 2 || damage < 0 || damage > 8191 || damage_target < -1)) return 2;
            tics = probe_tics;
            reference_sound_service(tics*2);
            MoveDoors();
            player->x = probe_x; player->y = probe_y;
            player->tilex = probe_x >> 16; player->tiley = probe_y >> 16;
            player->areanumber = planes[0][player->tiley*64+player->tilex]-AREATILE;
            plux = player->x >> 8; pluy = player->y >> 8;
            ConnectAreas();
            madenoise = probe_noise; player_damage = 0;
            if (damage) {
                objtype *target_ob = NULL;
                ordinal = 0;
                for (objtype *ob = player->next; ob; ob = ob->next) if (kind(ob) >= 0 && ordinal++ == damage_target) { target_ob = ob; break; }
                if (!target_ob || !(target_ob->flags & FL_SHOOTABLE)) return 2;
                DamageActor(target_ob, damage);
            }
            for (objtype *ob = player->next; ob; ob = ob->next) DoActor(ob);
            print_state();
        }
        return ferror(stdin) || ferror(stdout) ? 2 : 0;
    }
    int header, fast, shots, use_door, buttons, push_x, push_y, push_dir, raw_x, raw_y, entry_rng, input_best, input_weapon, input_ammo, input_chosen;
    int demo_commands = 0;
    const char *count_input = getenv("GDWOLF_DEMO_COMMAND_COUNT");
    if (count_input) {
        char *end;
        long count = strtol(count_input, &end, 10);
        if (!*count_input || *end || count < 1 || count > 21843) return 2;
        demo_commands = (int)count;
    }
    long px, py;
    while ((header = scanf("%ld %ld %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d", &px, &py, &entry_rng, &madenoise, &fast, &input_best, &input_weapon, &shots, &use_door, &buttons, &input_ammo, &input_chosen, &push_x, &push_y, &push_dir, &raw_x, &raw_y)) != EOF) {
        if (header != 17 || raw_x < -128 || raw_x > 127 || raw_y < -128 || raw_y > 127 || push_dir < -1 || push_dir > 3 || use_door < -1 || use_door >= doornum || shots < 0 || shots > 1 || buttons < 0 || buttons > 255 || gamestate.ammo < 0 || gamestate.ammo > 99 || gamestate.weapon < 0 || gamestate.weapon > 3 || px < 65536 || px >= 63L*65536 || py < 65536 || py >= 63L*65536 || rng_index < 0 || rng_index > 255) return 2;
        reference_command_index++;
        /* PollControls sets ex_completed on the final recorded input,
           before MoveDoors, player thinking and enemy damage can override it. */
        if (demo_commands && reference_command_index + 1 == demo_commands) playstate = ex_completed;
        controlx = raw_x*4; controly = raw_y*4;
        tics = 4;
        reference_sound_service(tics*2);
        MoveDoors(); MovePWalls();
        for (int i = 0; i < 64; i++) {
            int connected;
            if (scanf("%d", &connected) != 1) return 2;
            /* Diagnostic input only; ConnectAreas carries original connectivity. */
        }
        int changed;
        if (scanf("%d", &changed) != 1 || changed < 0 || changed > 4096) return 2;
        for (int i = 0; i < changed; i++) {
            int index, wall, area;
            if (scanf("%d %d %d", &index, &wall, &area) != 3 || index < 0 || index >= 4096 || wall < 0 || wall >= 90 || area < -1 || area >= 64) return 2;
            /* Diagnostic wall snapshot; MovePWalls carries original walls. */
        }
        for (int i = 0; i < doornum; i++) {
            int action, position;
            if (scanf("%d %d", &action, &position) != 2 || action < 0 || action > 3 || position < 0 || position > 65535) return 2;
            /* Diagnostic door snapshot; MoveDoors carries original state. */
        }
        /* Projections are evolved by original TransformActor/DrawScaleds.
           Consume the port snapshots for protocol diagnostics only. */
        for (objtype *ob = player->next; ob; ob = ob->next) if (kind(ob) >= 0) {
            int visible, screen;
            long trans;
            if (scanf("%d %d %ld", &visible, &screen, &trans) != 3 || visible < 0 || visible > 1) return 2;
        }
        for (int i = 0; i < 8; i++) {
            buttonheld[i] = buttonstate[i];
            buttonstate[i] = (buttons >> i) & 1;
        }
        reference_shots = 0; madenoise = 0;
        DoActor(player);
        tics = 4; player_damage = 0;
        for (objtype *ob = player->next; ob; ob = ob->next) DoActor(ob);
        print_state();
        int angle;
        if (scanf("%d", &angle) != 1) return feof(stdin) ? 0 : 2;
        if (angle != player->angle) Quit("render angle differs from original player angle");
        for (int y = 0; y < 64; y++) for (int x = 0; x < 64; x++) {
            int visible;
            if (scanf("%d", &visible) != 1 || visible < 0 || visible > 1) return 2;
            spotvis[x][y] = visible;
        }
        BuildTables(); CalcProjection(0x5700);
        viewcos = sintable[angle+90]; viewsin = sintable[angle];
        viewx = player->x - FixedByFrac(0x5700, viewcos);
        viewy = player->y + FixedByFrac(0x5700, viewsin);
        record_raycast();
        RefreshActorVisibility();
        reference_finish_tic();
        printf("{\"health\":%d,\"ammo\":%d,\"died\":%s,\"terminal\":%d,\"score\":%d,\"kills\":%d,\"stats\":", gamestate.health, gamestate.ammo, playstate == ex_died ? "true" : "false", playstate, gamestate.score, gamestate.killcount);
        print_stats();
        printf(",\"sound\":"); print_sound_state();
        printf(",\"victory\":"); print_victory();
        printf(",\"player_state\":%d", player->state == &s_deathcam ? 2 : player->state == &s_attack ? 1 : 0);
        print_static_slots();
        printf(",\"face\":{\"frame\":%d,\"timer\":%d},\"bonuses\":[", gamestate.faceframe, facecount);
        int printed = 0;
        for (statobj_t *spot = statobjlist; spot < laststatobj; spot++) if (spot->shapenum >= 0 && (spot->flags & FL_BONUS))
            printf("%s{\"x\":%d,\"y\":%d,\"shape\":%d}", printed++ ? "," : "", spot->tilex, spot->tiley, spot->shapenum);
        printf("],\"projections\":["); printed = 0;
        for (objtype *ob = player->next; ob; ob = ob->next) if (kind(ob) >= 0) {
            int shape = ob->state->shapenum;
            if (shape == -1) shape = ob->temp1;
            if (ob->state->rotate) shape += CalcRotate(ob);
            printf("%s{\"visible\":%s,\"view_x\":%d,\"trans_x\":%ld,\"shape\":%d}", printed++ ? "," : "", ob->flags & FL_VISABLE ? "true" : "false", ob->viewx, ob->transx, shape);
        }
        puts("]}"); fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
