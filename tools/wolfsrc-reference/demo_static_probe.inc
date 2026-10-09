/* Drive original static allocation and DrawScaleds pickup order directly.
   Every process starts with the original zero-initialized static storage;
   no resulting port state is supplied to the reference. */
static int static_probe(void) {
    mapwidth = mapheight = 64;
    for (int y = 0; y < 64; y++) {
        farmapylookup[y] = y*64;
        for (int x = 0; x < 64; x++) planes[0][y*64+x] = AREATILE;
    }
    InitActorList();
    BuildTables(); CalcProjection(0x5700);
    gamestate.difficulty = gd_hard;
    gamestate.health = 100; gamestate.ammo = 8;
    gamestate.lives = 3; gamestate.nextextra = EXTRAPOINTS;
    gamestate.weapon = gamestate.bestweapon = gamestate.chosenweapon = wp_pistol;
    initialize_reference_sound();
    char action[16];
    while (scanf("%15s", action) == 1) {
        if (!strcmp(action, "spawn") || !strcmp(action, "drop")) {
            int type, x, y;
            if (scanf("%d %d %d", &type, &x, &y) != 3 || x < 1 || x > 62 || y < 1 || y > 62) return 2;
            if (!strcmp(action, "spawn")) {
                if (type < 0 || type >= (int)(sizeof(statinfo)/sizeof(statinfo[0]))) return 2;
                SpawnStatic(x,y,type);
            } else {
                if (type < bo_gibs || type > bo_fullheal) return 2;
                PlaceItemType(type,x,y);
            }
        } else if (!strcmp(action, "bonus")) {
            int slot;
            if (scanf("%d", &slot) != 1 || slot < 0 || slot >= laststatobj-statobjlist) return 2;
            if (statobjlist[slot].shapenum == -1 || !(statobjlist[slot].flags&FL_BONUS)) return 2;
            GetBonus(&statobjlist[slot]);
        } else if (!strcmp(action, "inventory")) {
            int health, ammo;
            if (scanf("%d %d", &health, &ammo) != 2 || health < 0 || health > 100 || ammo < 0 || ammo > 99) return 2;
            gamestate.health = health; gamestate.ammo = ammo;
        } else if (!strcmp(action, "collect")) {
            long x, y;
            int angle;
            if (scanf("%ld %ld %d", &x, &y, &angle) != 3 || angle < 0 || angle >= 360 || x < TILEGLOBAL || x >= 63L*TILEGLOBAL || y < TILEGLOBAL || y >= 63L*TILEGLOBAL) return 2;
            player->x = x; player->y = y; player->angle = angle;
            memset(spotvis, 1, sizeof(spotvis));
            viewcos = sintable[angle+90]; viewsin = sintable[angle];
            viewx = x - FixedByFrac(0x5700,viewcos);
            viewy = y + FixedByFrac(0x5700,viewsin);
            RefreshActorVisibility();
        } else return 2;
        printf("{\"stats\":"); print_stats();
        printf(",\"sound\":"); print_sound_state();
        print_static_slots();
        puts("}");
        fflush(stdout);
    }
    return ferror(stdin) || ferror(stdout) ? 2 : 0;
}
