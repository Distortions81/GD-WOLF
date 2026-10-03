/* Host shim for raw-map actor initialization and face RNG, not an AI engine. */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define far
typedef int boolean;
enum { false, true };
enum { east, northeast, north, northwest, west, southwest, south, southeast, nodir };
enum { inertobj, guardobj, officerobj, ssobj, dogobj, bossobj, schabbobj,
       fakeobj, mechahitlerobj, mutantobj };
enum { gd_baby, gd_easy, gd_medium, gd_hard };
enum { NORTH, EAST, SOUTH, WEST, dr_closed, GETGATLINGSND };
#define TILESHIFT 16
#define TILEGLOBAL 65536
#define AREATILE 107
#define AMBUSHTILE 106
#define NUMENEMIES 22
#define SPDPATROL 512
#define SPDDOG 1500
#define FL_SHOOTABLE 1
#define FL_AMBUSH 64
typedef struct statestruct {
    int rotate, shapenum, tictime;
    void (*think)(void *), (*action)(void *);
    struct statestruct *next;
} statetype;
typedef struct {
    statetype *state;
    int ticcount, tilex, tiley, dir, areanumber, flags, obclass, hitpoints, active;
    long x, y, speed, distance;
} objtype;
typedef struct { int tilex, tiley, vertical, lock, action; } doorobj_t;
static struct { int difficulty, killtotal, secrettotal, faceframe; } gamestate;
static objtype objects[150], *new, *actorat[64][64];
static int object_count, mapwidth, mapheight, loadedgame;
static unsigned short planes[2][4096], *mapsegs[] = {planes[0], planes[1]};
static unsigned farmapylookup[64];
static unsigned char tilemap[64][64];
static doorobj_t doors[64], *lastdoorobj = doors;
static int doornum, doorposition[64], rng_index, tics, facecount, gatling_sound;
static long tileglobal = TILEGLOBAL;
static int US_RndT(void);
static void Quit(const char *message) { fprintf(stderr, "%s\n", message); exit(2); }
static void GetNewActor(void) {
    if (object_count >= 150) Quit("too many actors");
    new = &objects[object_count++];
}
/* Rendering/sound callbacks and spawn operations that consume no RNG. */
static void T_Stand(void *ob) { (void)ob; }
static void T_Path(void *ob) { (void)ob; }
static void DrawFace(void) {}
static int SD_SoundPlaying(void) { return gatling_sound ? GETGATLINGSND : -1; }
static void SpawnPlayer(int x, int y, int dir) { (void)x; (void)y; (void)dir; }
static void SpawnStatic(int x, int y, int type) { (void)x; (void)y; (void)type; }
/* Unsupported actor families fail explicitly rather than silently disappear. */
#define UNSUPPORTED(name) static void name(int x, int y) { (void)x; (void)y; Quit(#name " unsupported"); }
UNSUPPORTED(SpawnGretel)
UNSUPPORTED(SpawnGift)
UNSUPPORTED(SpawnFat)
UNSUPPORTED(SpawnSchabbs)
UNSUPPORTED(SpawnFakeHitler)
UNSUPPORTED(SpawnHitler)
static void SpawnGhosts(int which, int x, int y) { (void)which; (void)x; (void)y; Quit("ghosts unsupported"); }

#include "original_demo_start.inc"

static int US_RndT(void) { rng_index = (rng_index + 1) & 255; return rndtable[rng_index]; }

int main(void) {
    if (scanf("%d %d %d", &mapwidth, &mapheight, &gamestate.difficulty) != 3 ||
        mapwidth != 64 || mapheight != 64 || gamestate.difficulty < 0 || gamestate.difficulty > 3)
        Quit("invalid map header");
    for (int plane = 0; plane < 2; plane++)
        for (int i = 0; i < 4096; i++) {
            unsigned cell;
            if (scanf("%u", &cell) != 1 || cell > 65535) Quit("invalid map cell");
            planes[plane][i] = (unsigned short)cell;
        }
    /* SetupGameLevel's wall-copy loop; original SpawnDoor sets door areas. */
    for (int y = 0; y < 64; y++) {
        farmapylookup[y] = y * 64;
        for (int x = 0; x < 64; x++) {
            int tile = planes[0][y*64+x];
            if ((x == 0 || y == 0 || x == 63 || y == 63) && (tile == AMBUSHTILE || (tile >= 90 && tile <= 101)))
                Quit("ambush markers and doors require interior tiles");
            if ((x == 0 || y == 0 || x == 63 || y == 63) && planes[1][y*64+x] >= 108)
                Quit("actor markers require interior tiles");
            if (tile < AREATILE) {
                tilemap[x][y] = tile;
                actorat[x][y] = (objtype *)(uintptr_t)tile;
            }
        }
    }
    for (int y = 1; y < 63; y++)
        for (int x = 1; x < 63; x++) {
            int tile = planes[0][y*64+x];
            if (tile >= 90 && tile <= 101) SpawnDoor(x, y, !(tile & 1), (tile-90)/2);
            int info = planes[1][y*64+x];
            /* Original SpawnStand has no en_dog case. Don't execute its stale pointer path. */
            if ((info >= 134 && info <= 137) || (info >= 170 && info <= 173) || (info >= 206 && info <= 209))
                Quit("standing dog spawn unsupported in original source");
        }
    ScanInfoPlane();
    printf("{\"rng_index\":%d,\"actors\":[", rng_index);
    int printed = 0;
    for (int i = 0; i < object_count; i++) {
        objtype *ob = &objects[i];
        if (!(ob->flags & FL_SHOOTABLE)) continue;
        printf("%s{\"class\":%d,\"x\":%ld,\"y\":%ld,\"tile_x\":%d,\"tile_y\":%d,"
               "\"dir\":%d,\"area\":%d,\"ambush\":%s,\"health\":%d,\"speed\":%ld,"
               "\"distance\":%ld,\"shape\":%d,\"tic_count\":%d}", printed++ ? "," : "",
               ob->obclass, ob->x, ob->y, ob->tilex, ob->tiley, ob->dir, ob->areanumber,
               ob->flags & FL_AMBUSH ? "true" : "false", ob->hitpoints, ob->speed,
               ob->distance, ob->state->shapenum, ob->ticcount);
    }
    puts("]}"); fflush(stdout);
    /* Each call shares the caller's entry RNG index but retains facecount.
       This isolates UpdateFace from unverified actor/combat RNG consumers. */
    int seed, count;
    while ((count = scanf("%d %d %d", &tics, &seed, &gatling_sound)) != EOF) {
        if (count != 3 || tics < 0 || tics > 70 || seed < 0 || seed > 255 ||
            (gatling_sound != 0 && gatling_sound != 1)) Quit("invalid face request");
        rng_index = seed;
        UpdateFace();
        printf("{\"rng_index\":%d,\"face_count\":%d,\"face_frame\":%d}\n",
               rng_index, facecount, gamestate.faceframe);
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
