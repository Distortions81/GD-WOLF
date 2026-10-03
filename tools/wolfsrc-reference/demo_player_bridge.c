/* World snapshots come from GD-WOLF. Player position evolves independently.
 * This bridge compares player movement, not the original full simulation. */
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <math.h>

typedef int boolean;
typedef long fixed;
enum { false, true, bt_strafe = 1 };
#define MAPSIZE 64
#define ANGLES 360
#define ANGLEQUAD 90
#define PI 3.141592657
#define GLOBAL1 65536L
#define TILEGLOBAL GLOBAL1
#define TILESHIFT 16
#define MINDIST 0x5800L
#define PLAYERSIZE MINDIST
#define MINACTORDIST 0x10000L
#define AREATILE 107
#define EXITTILE 99
#define MOVESCALE 150L
#define BACKMOVESCALE 100L
#define ANGLESCALE 20
#define HITWALLSND 0
#define FL_SHOOTABLE 1

typedef struct { fixed x,y; unsigned tilex,tiley; int angle,areanumber,flags; } objtype;
static objtype objlist[151], *player = objlist, *actorat[64][64];
static unsigned short plane[2][4096], *mapsegs[2] = {plane[0],plane[1]};
static unsigned farmapylookup[64], mapwidth,mapheight;
static fixed sintable[451], *costable = sintable+90;
static struct { int victoryflag; } gamestate;
static int controlx,controly,anglefrac,buttonstate[8],noclip;
static long thrustspeed,playerxmove,playerymove;
static int SD_SoundPlaying(void) { return 0; }
static void SD_PlaySound(int sound) { (void)sound; }
static void VictoryTile(void) { gamestate.victoryflag = 1; }
static boolean TryMove(objtype *ob);
static void ClipMove(objtype *ob, long xmove, long ymove);
static void Thrust(int angle, long speed);

/* Exact integer equivalent of WL_DRAW.C's signed-magnitude FixedByFrac asm.
 * Only b's low 16 bits are multiplied; sign is its high bit, not two's complement. */
static fixed FixedByFrac(fixed a, fixed b) {
    int negative = (a < 0) ^ (((uint32_t)b & 0x80000000U) != 0);
    uint64_t magnitude = a < 0 ? (uint64_t)-a : (uint64_t)a;
    fixed result = (fixed)((magnitude * ((uint32_t)b & 0xffffU)) >> 16);
    return negative ? -result : result;
}

#include "original_player_movement.inc"

int main(void) {
    long x,y;
    int angle,width,height;
    if (scanf("%ld %ld %d %d %d", &x,&y,&angle,&width,&height) != 5 ||
        width < 3 || width > 64 || height < 3 || height > 64 ||
        angle < 0 || angle >= 360 || x < GLOBAL1 || y < GLOBAL1 ||
        x >= (width-1)*GLOBAL1 || y >= (height-1)*GLOBAL1) return 2;
    player->x=x; player->y=y; player->angle=angle;
    mapwidth=width; mapheight=height;
    for (int row=0;row<64;row++) {
        farmapylookup[row]=row*64;
        for (int col=0;col<64;col++) plane[0][row*64+col]=AREATILE;
    }
    BuildMovementTables();
    int buttons,cx,cy,count,header;
    while ((header=scanf("%d %d %d %d", &buttons,&cx,&cy,&count)) != EOF) {
        if (header != 4 || buttons<0 || buttons>255 || cx < -128 || cx>127 ||
            cy < -128 || cy>127 || count<0 || count>150) return 2;
        for (int row=0;row<64;row++) for (int col=0;col<64;col++)
            actorat[col][row]=(objtype *)(uintptr_t)1;
        for (int row=0;row<height;row++) for (int col=0;col<width;col++) {
            int cell;
            if (scanf("%d",&cell)!=1 || (cell!=0 && cell!=1) ||
                ((row==0 || col==0 || row==height-1 || col==width-1) && cell!=1)) return 2;
            actorat[col][row]=(objtype *)(uintptr_t)cell;
        }
        for (int i=1;i<=count;i++) {
            int tx,ty;
            if (scanf("%d %d %ld %ld",&tx,&ty,&x,&y)!=4 || tx<0 || tx>=width || ty<0 || ty>=height) return 2;
            objlist[i].tilex=tx; objlist[i].tiley=ty;
            objlist[i].x=x; objlist[i].y=y; objlist[i].flags=1;
            actorat[tx][ty]=&objlist[i];
        }
        for (int i=0;i<8;i++) buttonstate[i]=(buttons>>i)&1;
        controlx=cx*4; controly=cy*4;
        ControlMovement(player);
        printf("{\"x\":%ld,\"y\":%ld,\"angle\":%d,\"angle_frac\":%d}\n",
               player->x,player->y,player->angle,anglefrac);
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
