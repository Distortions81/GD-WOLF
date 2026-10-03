/* Host/protocol shim. AI functions come from generated original_movement.inc. */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef int boolean;
enum { false, true };
typedef enum { east, northeast, north, northwest, west, southwest,
               south, southeast, nodir } dirtype;
enum { inertobj, guardobj, dogobj, fakeobj };
#define FL_SHOOTABLE 1
#define FL_FIRSTATTACK 32
#define AREATILE 107
#define TILEGLOBAL 65536
typedef struct {
    int tilex, tiley, obclass, flags, areanumber;
    dirtype dir;
    long distance;
} objtype;

/* A wall border permits the original unchecked neighbor indexing safely. */
static objtype *actorat[64][64];
static unsigned short plane[64 * 64], *mapsegs[] = {plane};
static unsigned farmapylookup[64];
static objtype player_storage, blocker, *player = &player_storage;
static unsigned rng_index;
static int opened_door;
static int US_RndT(void);
static void OpenDoor(int door) { opened_door = door; }
static void Quit(const char *message) { fprintf(stderr, "%s\n", message); exit(2); }
static boolean TryWalk(objtype *ob);

#include "original_movement.inc"

/* ID_US_A.ASM increments and wraps the index before reading the table. */
static int US_RndT(void) { rng_index = (rng_index + 1) & 255; return rndtable[rng_index]; }

int main(void) {
    int mode, dir, seed, dog, first, x, y, px, py, width, height;
    int header;
    while ((header = scanf("%d %d %d %d %d %d %d %d %d %d %d",
           &mode, &dir, &seed, &dog, &first, &x, &y, &px, &py, &width, &height)) != EOF) {
        if (header != 11 || mode < 0 || mode > 3 || dir < 0 || dir > 8 ||
            seed < 0 || seed > 255 || (dog != 0 && dog != 1) ||
            (first != 0 && first != 1) || width < 3 || width > 64 || height < 3 || height > 64 ||
            x < 1 || x >= width-1 || y < 1 || y >= height-1 ||
            px < 0 || px >= width || py < 0 || py >= height) {
            fprintf(stderr, "invalid reference request header\n"); return 2;
        }
        for (int row = 0; row < 64; row++) {
            farmapylookup[row] = row * 64;
            for (int col = 0; col < 64; col++) {
                actorat[col][row] = (objtype *)(uintptr_t)1;
                plane[row * 64 + col] = AREATILE;
            }
        }
        blocker.flags = FL_SHOOTABLE;
        for (int row = 0; row < height; row++) {
            for (int col = 0; col < width; col++) {
                int cell;
                if (scanf("%d", &cell) != 1 ||
                    (cell != 0 && cell != 1 && cell != 256 && (cell < 128 || cell > 191))) {
                    fprintf(stderr, "invalid reference map cell\n"); return 2;
                }
                if ((row == 0 || col == 0 || row == height-1 || col == width-1) && cell != 1) {
                    fprintf(stderr, "reference maps require a solid border\n"); return 2;
                }
                actorat[col][row] = cell == 256 ? &blocker : (objtype *)(uintptr_t)cell;
            }
        }
        objtype ob = {0};
        ob.tilex = x; ob.tiley = y; ob.dir = (dirtype)dir;
        ob.obclass = dog ? dogobj : guardobj;
        ob.flags = FL_SHOOTABLE | (first ? FL_FIRSTATTACK : 0);
        player->tilex = px; player->tiley = py;
        rng_index = (unsigned)seed; opened_door = -1;
        boolean moved;
        if (mode == 3) moved = TryWalk(&ob);
        else {
            if (mode == 0) SelectChaseDir(&ob);
            else if (mode == 1) SelectDodgeDir(&ob);
            else SelectRunDir(&ob);
            moved = ob.dir != nodir;
        }
        printf("{\"dir\":%d,\"x\":%d,\"y\":%d,\"move\":%s,\"wait\":%s,"
               "\"rng_index\":%u,\"first_attack\":%s,\"opened_door\":%d}\n",
               (int)ob.dir, ob.tilex, ob.tiley, moved ? "true" : "false",
               ob.distance < 0 ? "true" : "false", rng_index,
               (ob.flags & FL_FIRSTATTACK) ? "true" : "false", opened_door);
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
