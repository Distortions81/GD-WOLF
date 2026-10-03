/* Original actors evolve independently; player, door and renderer inputs
   are external. This is an actor-runtime comparator, not a full engine. */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#define far
#define UPLOAD
typedef int boolean;
enum { false, true };
enum { gd_baby, gd_easy, gd_medium, gd_hard };
enum { NORTH, EAST, SOUTH, WEST };
enum { dr_closed, dr_open, dr_opening, dr_closing };
enum { wp_knife, wp_pistol, wp_machinegun, wp_chaingun };
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
#define MAPSPOT(x,y,p) (mapsegs[p][(y)*64+(x)])
#include "original_demo_ai_types.inc"
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
};
typedef struct { int tilex, tiley, vertical, lock, action, ticcount; } doorobj_t;
static struct { int difficulty, killtotal, secrettotal, victoryflag, bestweapon, killcount; long killx, killy; } gamestate;
static objtype objects[150], *new, *actorat[64][64], player_storage, *player = &player_storage;
static int object_count, mapwidth, mapheight, loadedgame, rng_index, tics, madenoise;
static unsigned short planes[2][4096], *mapsegs[] = {planes[0], planes[1]};
static unsigned farmapylookup[64], plux, pluy;
static unsigned char tilemap[64][64];
static doorobj_t doorobjlist[64], *lastdoorobj = doorobjlist;
static int doornum, areabyplayer[64], doorposition[64], thrustspeed, player_damage;
static long tileglobal = TILEGLOBAL;
static char str[256];
static int US_RndT(void);
static void SpawnStatic(int x, int y, int type);
static boolean TryWalk(objtype *ob);
static void Quit(const char *message) { fprintf(stderr, "%s\n", message); exit(2); }
static void GetNewActor(void) {
    if (object_count >= 150) Quit("too many actors");
    new = &objects[object_count++];
}
static void OpenDoor(int door) { if (door < 0 || door >= doornum) Quit("bad door"); }
static void TakeDamage(int damage, objtype *ob) { (void)ob; player_damage += damage; }
static void PlaySoundLocActor(int sound, objtype *ob) { (void)sound; (void)ob; }
static void SD_PlaySound(int sound) { (void)sound; }
static void PlaceItemType(stat_t type, int x, int y) { (void)type; (void)x; (void)y; }
static void GivePoints(int points) { (void)points; }
static void A_StartDeathCam(objtype *ob) { (void)ob; Quit("boss death camera unsupported"); }
static void RemoveObj(objtype *ob) { ob->state = NULL; }
static void SpawnPlayer(int x, int y, int dir) { (void)x; (void)y; (void)dir; }
#define UNSUPPORTED(name) static void name(int x, int y) { (void)x; (void)y; Quit(#name " unsupported"); }
UNSUPPORTED(SpawnGretel)
UNSUPPORTED(SpawnGift)
UNSUPPORTED(SpawnFat)
UNSUPPORTED(SpawnSchabbs)
UNSUPPORTED(SpawnFakeHitler)
UNSUPPORTED(SpawnHitler)
static void SpawnGhosts(int which, int x, int y) { (void)which; (void)x; (void)y; Quit("ghosts unsupported"); }
#include "original_demo_ai.inc"
static int US_RndT(void) { rng_index = (rng_index + 1) & 255; return rndtable[rng_index]; }
static void SpawnStatic(int x, int y, int type) {
    if (type < 0 || type >= 48) Quit("unsupported static");
    if (statinfo[type].type == block) actorat[x][y] = (objtype *)(uintptr_t)1;
}
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
    puts("]}"); fflush(stdout);
}
int main(void) {
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
    for (int y = 1; y < 63; y++) for (int x = 1; x < 63; x++) if (tilemap[x][y] == AMBUSHTILE) {
        tilemap[x][y] = 0;
        if ((uintptr_t)actorat[x][y] == AMBUSHTILE) actorat[x][y] = NULL;
    }
    print_state();
    int header, fast;
    while ((header = scanf("%ld %ld %d %d %d %d", &player->x, &player->y, &rng_index, &madenoise, &fast, &gamestate.bestweapon)) != EOF) {
        if (header != 6 || player->x < 65536 || player->x >= 63L*65536 || player->y < 65536 || player->y >= 63L*65536 || rng_index < 0 || rng_index > 255) return 2;
        player->tilex = player->x >> 16; player->tiley = player->y >> 16;
        plux = player->x >> 8; pluy = player->y >> 8; thrustspeed = fast ? RUNSPEED : 0;
        for (int i = 0; i < 64; i++) if (scanf("%d", &areabyplayer[i]) != 1) return 2;
        for (int i = 0; i < doornum; i++) {
            int action, position;
            if (scanf("%d %d", &action, &position) != 2 || action < 0 || action > 3 || position < 0 || position > 65535) return 2;
            doorobjlist[i].action = action; doorposition[i] = position;
            int x = doorobjlist[i].tilex, y = doorobjlist[i].tiley;
            if (action == dr_open && (uintptr_t)actorat[x][y] < 256) actorat[x][y] = NULL;
            if (action != dr_open && !actorat[x][y]) actorat[x][y] = (objtype *)(uintptr_t)(i|128);
        }
        /* External damage amounts are deliberately separate from AI updates. */
        for (int i = 0; i < object_count; i++) if (kind(&objects[i]) >= 0) {
            int damage, visible;
            if (scanf("%d %d", &damage, &visible) != 2 || damage < -1 || damage > 65535) return 2;
            objects[i].flags = (objects[i].flags & ~FL_VISABLE) | (visible ? FL_VISABLE : 0);
            if (visible) objects[i].active = true; /* DrawScaleds activation. */
            if (damage >= 0) DamageActor(&objects[i], damage);
        }
        tics = 4; player_damage = 0;
        for (int i = 0; i < object_count; i++) if (kind(&objects[i]) >= 0) DoActor(&objects[i]);
        print_state();
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
