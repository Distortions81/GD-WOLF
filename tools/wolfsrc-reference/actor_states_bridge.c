/* Original scheduler; callbacks record invocation timing, without running AI. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
enum { false, true };
#define FL_NONMARK 128
#define FL_NEVERMARK 4
typedef struct object objtype;
typedef struct statestruct {
    int rotate, shapenum, tictime;
    void (*think)(objtype *), (*action)(objtype *);
    struct statestruct *next;
} statetype;
struct object { int active, areanumber, flags, tilex, tiley, ticcount; statetype *state; };
static objtype ob, *actorat[64][64];
static int areabyplayer[64], tics, thought, shots, bites;
static void T_Stand(objtype *a) { (void)a; thought = 1; }
static void T_Path(objtype *a) { (void)a; thought = 1; }
static void T_Chase(objtype *a) { (void)a; thought = 1; }
static void T_DogChase(objtype *a) { (void)a; thought = 1; }
static void T_Shoot(objtype *a) { (void)a; shots++; }
static void T_Bite(objtype *a) { (void)a; bites++; }
static void RemoveObj(objtype *a) { a->state = NULL; }

#include "original_actor_states.inc"

static statetype *starts[6][5] = {
    {&s_grdstand, &s_grdpath1, &s_grdchase1, &s_grdshoot1, &s_grdpain},
    {&s_ofcstand, &s_ofcpath1, &s_ofcchase1, &s_ofcshoot1, &s_ofcpain},
    {&s_ssstand, &s_sspath1, &s_sschase1, &s_ssshoot1, &s_sspain},
    {NULL, &s_dogpath1, &s_dogchase1, &s_dogjump1, NULL},
    {&s_bossstand, NULL, &s_bosschase1, &s_bossshoot1, NULL},
    {&s_mutstand, &s_mutpath1, &s_mutchase1, &s_mutshoot1, &s_mutpain}
};
int main(void) {
    char op[16];
    while (scanf("%15s", op) == 1) {
        if (!strcmp(op, "reset")) {
            int kind, mode, remaining;
            if (scanf("%d %d %d", &kind, &mode, &remaining) != 3 ||
                kind < 0 || kind >= 6 || mode < 0 || mode >= 5 ||
                !starts[kind][mode] || remaining < 0 || remaining > starts[kind][mode]->tictime)
                return 2;
            memset(&ob, 0, sizeof(ob));
            ob.state = starts[kind][mode]; ob.ticcount = remaining;
            ob.active = 1; ob.tilex = ob.tiley = 2;
            shots = bites = 0;
        } else if (!strcmp(op, "step")) {
            if (scanf("%d", &tics) != 1 || tics < 1 || tics > 70 || !ob.state) return 2;
            thought = 0;
            DoActor(&ob);
            printf("{\"shape\":%d,\"tic_count\":%d,\"think\":%s,\"shots\":%d,\"bites\":%d}\n",
                   ob.state->shapenum, ob.ticcount, thought ? "true" : "false", shots, bites);
            fflush(stdout);
        } else return 2;
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
