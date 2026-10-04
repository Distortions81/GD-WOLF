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
enum { dr_closed, dr_open, dr_opening, dr_closing };
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
#define FINEANGLES 3600
#define ANGLES 360
#define ANGLEQUAD 90
#define GLOBAL1 65536
#define VIEWGLOBAL 65536
#define MINDIST 0x5800
#define ACTORSIZE 0x4000
#define NUMAREAS 64
#define OPENTICS 300
#define MAXSTATS 400
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
    int ticcount, tilex, tiley, areanumber, flags, obclass, hitpoints, active, temp2;
    dirtype dir;
    long x, y, speed, distance;
    long transx;
    int viewx;
    int viewheight;
    int angle;
    int temp1;
    fixed transy;
    objtype *next;
};
typedef struct { int tilex, tiley, vertical, lock, action, ticcount; } doorobj_t;
static struct { int difficulty, killtotal, secrettotal, victoryflag, bestweapon, weapon, killcount, keys, ammo, chosenweapon, attackcount, attackframe, weaponframe, secretcount, health, treasuretotal, treasurecount, lives, faceframe; long killx, killy; } gamestate;
static objtype objects[150], *new, *actorat[64][64], player_storage, *player = &player_storage;
static int object_count, mapwidth, mapheight, loadedgame, rng_index, tics, madenoise;
static unsigned short planes[2][4096], *mapsegs[] = {planes[0], planes[1]};
static unsigned farmapylookup[64], plux, pluy;
static unsigned char tilemap[64][64];
static doorobj_t doorobjlist[64], *lastdoorobj = doorobjlist;
static int doornum, areabyplayer[64], doorposition[64], thrustspeed, player_damage;
static unsigned char areaconnect[64][64];
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
static int CalcRotate(objtype *ob) { (void)ob; return 0; }
static long tileglobal = TILEGLOBAL;
static char str[256];
static int centerx = 119, shootdelta = 24;
static int viewwidth = 240, pixelangle[240], minheightdiv;
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
static void GetNewActor(void) {
    if (object_count >= 150) Quit("too many actors");
    new = &objects[object_count++];
    if (object_count == 1) player->next = new;
    else objects[object_count-2].next = new;
}
static void OpenDoor(int door);
static void ConnectAreas(void);
static void TakeDamage(int points, objtype *attacker);
static void StartDamageFlash(int points) { (void)points; }
static void PlaySoundLocActor(int sound, objtype *ob) { (void)sound; (void)ob; }
static void PlaySoundLocTile(int sound, int x, int y) { (void)sound; (void)x; (void)y; }
static void SD_PlaySound(int sound) { (void)sound; }
static void SD_WaitSoundDone(void) {}
static void DrawWeapon(void) {}
static void DrawAmmo(void) {}
static void DrawHealth(void) {}
static void DrawFace(void) {}
static void DrawKeys(void) {}
static void DrawLives(void) {}
static void StartBonusFlash(void) {}
static void StatusDrawPic(int x, int y, int pic) { (void)x; (void)y; (void)pic; }
static void UpdateFace(void) { OriginalUpdateFace(); }
static void ControlMovement(objtype *ob) { OriginalControlMovement(ob); }
static int SD_SoundPlaying(void) { return -1; } /* No active sound matches the stub IDs. */
static void VictoryTile(void) { Quit("victory tile unsupported"); }
static void VictorySpin(void) { Quit("victory spin unsupported"); }
static void PlaceItemType(int type, int x, int y);
static void GivePoints(int points) { (void)points; }
static void A_StartDeathCam(objtype *ob) { (void)ob; Quit("boss death camera unsupported"); }
static void RemoveObj(objtype *ob) { ob->state = NULL; }
static void SpawnPlayer(int x, int y, int dir) {
    (void)dir; player->x = x*65536L+32768; player->y = y*65536L+32768;
    player->tilex = x; player->tiley = y; player->areanumber = planes[0][y*64+x]-AREATILE;
    player->angle = ((1-dir)*90+360)%360;
}
#define UNSUPPORTED(name) static void name(int x, int y) { (void)x; (void)y; Quit(#name " unsupported"); }
UNSUPPORTED(SpawnGretel)
UNSUPPORTED(SpawnGift)
UNSUPPORTED(SpawnFat)
UNSUPPORTED(SpawnSchabbs)
UNSUPPORTED(SpawnFakeHitler)
UNSUPPORTED(SpawnHitler)
static void SpawnGhosts(int which, int x, int y) { (void)which; (void)x; (void)y; Quit("ghosts unsupported"); }
#include "original_demo_runtime.inc"
static int US_RndT(void) { rng_index = (rng_index + 1) & 255; return rndtable[rng_index]; }
static int kind(objtype *ob) {
    switch (ob->obclass) {
        case guardobj: return 0; case officerobj: return 1; case ssobj: return 2;
        case dogobj: return 3; case bossobj: return 4; case mutantobj: return 5;
        default: return -1;
    }
}
static void print_state(void) {
    printf("{\"rng_index\":%d,\"damage\":%d,\"actors\":[", rng_index, player_damage);
    int printed = 0;
    for (int i = 0; i < object_count; i++) {
        objtype *ob = &objects[i];
        if (kind(ob) < 0) continue;
        printf("%s{\"kind\":%d,\"x\":%ld,\"y\":%ld,\"tile_x\":%d,\"tile_y\":%d,\"dir\":%d,"
               "\"area\":%d,\"distance\":%ld,\"reaction\":%d,\"health\":%d,\"flags\":%d,\"shape\":%d,\"tic_count\":%d}",
               printed++ ? "," : "", kind(ob), ob->x, ob->y, ob->tilex, ob->tiley, ob->dir,
               ob->areanumber, ob->distance, ob->temp2, ob->hitpoints > 0 ? ob->hitpoints : 0,
               ob->flags & (FL_SHOOTABLE|FL_ATTACKMODE|FL_FIRSTATTACK|FL_AMBUSH), ob->state->shapenum, ob->ticcount);
    }
    printf("],\"doors\":[");
    for (int i = 0; i < doornum; i++) printf("%s{\"action\":%d,\"position\":%d,\"timer\":%d}", i ? "," : "", doorobjlist[i].action, doorposition[i], doorobjlist[i].action == dr_open ? doorobjlist[i].ticcount : 0);
    printf("],\"walls\":[");
    for (int y = 0; y < 64; y++) for (int x = 0; x < 64; x++) printf("%s%d", x || y ? "," : "", tilemap[x][y] && !(tilemap[x][y] >= 128 && tilemap[x][y] < 192));
    printf("],\"weapon\":{\"type\":%d,\"attacking\":%s,\"frame\":%d,\"timer\":%d,\"ammo\":%d,\"shots\":%d},\"player\":{\"x\":%ld,\"y\":%ld,\"angle\":%d,\"angle_frac\":%d}}\n", gamestate.weapon, player->state == &s_attack ? "true" : "false", gamestate.attackframe, player->state == &s_attack ? gamestate.attackcount : 0, gamestate.ammo, reference_shots, player->x, player->y, player->angle, anglefrac);
    fflush(stdout);
}
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
int main(int argc, char **argv) {
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
    if (argc != 1) return 2;
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
    ScanInfoPlane();
    BuildTables();
    player->state = &s_player; gamestate.ammo = 8; gamestate.health = 100;
    InitAreas();
    for (int y = 1; y < 63; y++) for (int x = 1; x < 63; x++) if (planes[0][y*64+x] == AMBUSHTILE) {
        tilemap[x][y] = 0;
        if ((uintptr_t)actorat[x][y] == AMBUSHTILE) actorat[x][y] = NULL;
    }
    gamestate.weapon = gamestate.bestweapon = gamestate.chosenweapon = 1;
    print_state();
    int header, fast, shots, use_door, buttons, push_x, push_y, push_dir, raw_x, raw_y, entry_rng, input_best, input_weapon, input_ammo, input_chosen;
    long px, py;
    while ((header = scanf("%ld %ld %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d", &px, &py, &entry_rng, &madenoise, &fast, &input_best, &input_weapon, &shots, &use_door, &buttons, &input_ammo, &input_chosen, &push_x, &push_y, &push_dir, &raw_x, &raw_y)) != EOF) {
        if (header != 17 || raw_x < -128 || raw_x > 127 || raw_y < -128 || raw_y > 127 || push_dir < -1 || push_dir > 3 || use_door < -1 || use_door >= doornum || shots < 0 || shots > 1 || buttons < 0 || buttons > 255 || gamestate.ammo < 0 || gamestate.ammo > 99 || gamestate.weapon < 0 || gamestate.weapon > 3 || px < 65536 || px >= 63L*65536 || py < 65536 || py >= 63L*65536 || rng_index < 0 || rng_index > 255) return 2;
        controlx = raw_x*4; controly = raw_y*4;
        tics = 4;
        MoveDoors(); MovePWalls();
        player->tilex = player->x >> 16; player->tiley = player->y >> 16;
        player->areanumber = planes[0][player->tiley*64+player->tilex]-AREATILE;
        plux = player->x >> 8; pluy = player->y >> 8; thrustspeed = fast ? RUNSPEED : 0;
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
        for (int i = 0; i < object_count; i++) if (kind(&objects[i]) >= 0) {
            int visible, screen;
            long trans;
            if (scanf("%d %d %ld", &visible, &screen, &trans) != 3 || visible < 0 || visible > 1) return 2;
        }
        for (int i = 0; i < 8; i++) {
            buttonheld[i] = buttonstate[i];
            buttonstate[i] = (buttons >> i) & 1;
        }
        reference_shots = 0; madenoise = 0;
        if (player->state == &s_attack) T_Attack(player);
        else T_Player(player);
        player->tilex = player->x >> 16; player->tiley = player->y >> 16;
        plux = player->x >> 8; pluy = player->y >> 8;
        tics = 4; player_damage = 0;
        for (int i = 0; i < object_count; i++) if (kind(&objects[i]) >= 0) DoActor(&objects[i]);
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
        printf("{\"health\":%d,\"ammo\":%d,\"died\":%s,\"projections\":[", gamestate.health, gamestate.ammo, playstate == ex_died ? "true" : "false"); int printed = 0;
        for (int i = 0; i < object_count; i++) if (kind(&objects[i]) >= 0) {
            printf("%s{\"visible\":%s,\"view_x\":%d,\"trans_x\":%ld}", printed++ ? "," : "", objects[i].flags & FL_VISABLE ? "true" : "false", objects[i].viewx, objects[i].transx);
        }
        puts("]}"); fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
