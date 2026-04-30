# Enemy Parity Checklist

This document tracks enemy parity work against `WOLFSRC`, with the current focus narrowed to the Wolf3D shareware release and especially Episode 1.

Source references:
- Spawn roster and map object IDs: `WOLFSRC/WL_GAME.C`
- Enemy enums and spawn entry points: `WOLFSRC/WL_DEF.H`
- Core actor logic for regular enemies and bosses: `WOLFSRC/WL_ACT2.C`
- Current port actor implementation: `actor_ai.go`
- Current port actor definitions and sequences: `sprite_catalog.go`

## Scope

Current implementation focus:
- Lowest-tier to highest-tier enemies
- Mark progress in this document as code lands

Parity standard for this document:
- Treat `WOLFSRC` as the gameplay authority for enemy behavior
- Chase source-matching behavior, not just approximate feel
- Count edge cases as parity work, not optional polish
- Do not mark an enemy `[done]` if its runtime behavior still knowingly diverges in combat, movement, door interaction, sound triggering, or state timing
- Prefer direct source-backed tests for attack start conditions, chase selection, damage rules, sound timing, death outcomes, and map interaction whenever practical

Out of scope for this pass:
- Spear of Destiny-only enemies
- Later-episode Wolf3D bosses until Episode 1 coverage is in better shape

## Progress Legend

- `[done]` source-backed baseline with no known material gameplay divergence in the current target scope
- `[partial]` some scaffolding exists, but gameplay parity is incomplete
- `[next]` intended near-term implementation target
- `[later]` real Wolf3D enemy, but not first target for shareware Episode 1
- `[n/a]` not present in shareware Episode 1 enemy progression

For this document, "material gameplay divergence" includes:
- different chase-direction or dodge behavior
- different attack start conditions or hit rules
- different door interaction or movement blocking behavior
- different sequence timing or frame actions
- different alert / attack / death sound triggering
- different drop, score, or death outcomes

## Current Port Snapshot

Implemented in gameplay:
- `[done]` Guard
- `[done]` Dog
- `[done]` Officer
- `[done]` SS
- `[done]` Hans Grosse
- `[partial]` Mutant
- `[done]` Dead guard corpse content

Missing from gameplay:
- `[later]` Gretel Grosse
- `[later]` Dr. Schabbs
- `[later]` Giftmacher
- `[later]` Fatface
- `[later]` Fake Hitler
- `[later]` Mecha-Hitler / Hitler
- `[later]` Ghosts: Blinky, Clyde, Pinky, Inky

## Shareware Episode 1 Track

This is the working order for the document and implementation plan.

### 1. Guard

Shareware Episode 1 relevance:
- Core baseline enemy
- Present from the beginning

Current status:
- `[done]` Spawned and updated in gameplay
- `[done]` Stand, patrol, chase, pain, shoot, and death flow exist in the port with source-backed shareware behavior coverage

Remaining parity gaps:
- none in the current shareware Episode 1 target scope

Notes:
- Guard remains the reference baseline for later humanoid enemy work
- Do not consider it "finished forever"; use it as the standard to compare officer and SS behavior against
- Keep revisiting guard when a source-level mismatch is found elsewhere; if the baseline is wrong, later parity claims are weak
- Earlier `[done]` status here was too optimistic; guard is the current audit target, not a finished parity baseline

### 2. Dog

Shareware Episode 1 relevance:
- Low-tier enemy, but distinct from guards
- Present early enough to keep in the first-wave parity pass

Current status:
- `[done]` Spawned and updated in gameplay
- `[done]` Dedicated dog chase and jump/bite behavior exists
- `[done]` Dedicated dog death sequence exists
- `[done]` `T_DogChase`/`T_Bite` edge cases now have direct regression coverage for live gameplay spacing, tic-scaled jump entry, bite sound-on-miss, and bite resolution after jump starts

Remaining parity gaps:
- none in the current shareware Episode 1 target scope

Notes:
- Dog is currently the strongest "non-guard" parity baseline in the port

### 3. Officer

Shareware Episode 1 relevance:
- Mid-tier humanoid enemy in the shareware roster
- First immediate gap after guard and dog

Current status:
- `[done]` active in gameplay with source-aligned spawn, sequence, stat, and sound coverage for the shareware target
- `[done]` attack sequence timing matches `s_ofcshoot1` through `s_ofcshoot3`

Landed so far:
- officer spawn filtering has been removed from the active actor pipeline
- officer sequence coverage now mirrors the basic `WOLFSRC` officer state set
- officer chase speed now follows the original "faster than guard" pattern
- officer sight/death sound hooks now map to `SPIONSND` / `NEINSOVASSND`
- officer attack frames are now covered by explicit timing tests against the source sequence

Reason to do this first:
- closest to guard logic
- cheapest parity win after current baselines
- useful as the first expansion of actor dispatch beyond guard and dog

Remaining parity gaps:
- none in the current shareware Episode 1 target scope

### 4. SS

Shareware Episode 1 relevance:
- High-tier humanoid enemy in the shareware roster
- Still part of Episode 1 progression, but should follow officer

Current status:
- `[done]` active in gameplay with source-aligned spawn, burst-fire sequence, sounds, score, and machinegun drop behavior for the shareware target
- `[done]` burst-fire sequence matches `s_ssshoot1` through `s_ssshoot9`
- `[done]` stronger shooting path from `T_Shoot` is now represented by the SS/boss accuracy adjustment

Reason to do this second:
- still humanoid and shares the same broad AI shell
- adds a more interesting attack cadence without requiring the projectile system yet

Remaining parity gaps:
- none in the current shareware Episode 1 target scope

### 5. Hans Grosse

Shareware Episode 1 relevance:
- Episode 1 boss
- first boss worth targeting once the regular shareware roster is in place

Current status:
- `[done]` Hans now spawns from shareware map object `214`
- `[done]` dedicated stand, chase, shoot, and death sequences now exist and match the source timing baseline
- `[done]` boss-specific alert, attack, and death sounds now exist
- `[done]` Hans hit points and score now follow the shareware/source baseline used by the port difficulty model
- `[done]` Hans death now drops the Episode 1 gold key, matching `KillActor(bossobj)`
- `[done]` Hans spawn/chase presentation now matches the source baseline more closely: `SpawnBoss()` always starts him facing south in ambush, and his stand/chase/shoot flow stays non-rotating like `s_bossstand` through `s_bossshoot8`

Remaining parity gaps:
- none in the current shareware Episode 1 target scope

Reason not to do this first:
- requires more bespoke state flow than officer/SS
- boss-death consequences should land after regular actor dispatch is broadened

### Shareware Cleanup

#### Dead guard corpse

Shareware relevance:
- present in shareware map object data
- simple parity item that should not remain missing while Episode 1 work is active

Current status:
- `[done]` map object `124` now builds as a non-blocking corpse sprite using the guard dead frame
- `[done]` active humanoid spawn tiles are no longer duplicated into `staticSprites`

Notes:
- this is intentionally treated as static map content, matching the inert `SpawnDeadGuard()` path in `WOLFSRC`

## Shareware Episode 1 Immediate Work Order

This is the active implementation ladder for the shareware pass.

1. `[done]` Guard baseline
2. `[done]` Dog baseline
3. `[done]` Officer finish pass
4. `[done]` SS finish pass
5. `[done]` Hans Grosse

## Later Backlog

These remain in scope for full parity, but they are intentionally separated from the current shareware Episode 1 work.

Use this section as the holding area for:
- later-episode Wolf3D enemies
- non-Episode 1 bosses
- enemies that depend on bigger engine work not needed for the current pass

Nothing here is removed from parity planning. It is only deferred.

### Later Wolf3D Regular Enemies

#### Mutant
- `[partial]` active gameplay support now covers map spawn mapping, stand/patrol/chase/pain/shoot/death sequences, difficulty HP, no-alert first sighting, default fire sound, AHHHG death sound, and clip drop behavior
- `[partial]` medium/hard mutant map object ranges are now active in gameplay instead of definition-only
- next pass should finish direct source audit of mutant-specific attack cadence, movement edge cases, and any later-episode map interactions before marking it done

#### Dead guard corpse
- `[done]` handled in the shareware pass

#### Ghosts
- `[later]` require special movement/collision assumptions
- not useful to start with while humanoid roster is still incomplete

### Later Wolf3D Bosses

These stay visible here so we do not lose sequencing after the Episode 1-first pass.

#### Gretel Grosse
- `[later]` same broad family as Hans, but not Episode 1-first work

#### Dr. Schabbs
- `[later]` depends on projectile support

#### Giftmacher
- `[later]` depends on projectile support

#### Fatface
- `[later]` later boss pass

#### Fake Hitler
- `[later]` requires special attack/transform handling

#### Hitler / Mecha-Hitler
- `[later]` requires two-stage boss handling and strong death/event parity

## Cross-Cutting Engine Gaps

These are the codebase constraints that currently block moving from guard/dog to the rest of the roster.

### 1. Actor instantiation and dispatch

Current gap:
- active gameplay now covers guard, officer, SS, dog, and Hans
- later roster entries still stop at definition-only coverage

Needed:
- broaden spawn/update support again when mutant and later bosses move into scope

Episode 1 value:
- this blocker is cleared for the full shareware Episode 1 roster

### 2. Per-enemy sequences

Current gap:
- shareware humanoids now have sequence coverage, but later enemies still do not

Needed:
- complete stand, patrol, chase, pain, shoot, and death coverage for later roster entries when they come into scope

Episode 1 value:
- this blocker is cleared for the shareware Episode 1 roster

### 3. Per-enemy combat logic

Current gap:
- shareware ranged combat now has distinct officer, SS, and Hans coverage

Needed:
- extend the same combat treatment to later roster entries when they come into scope

Episode 1 value:
- this is the main differentiator after spawn support

## Shareware Audit Notes

Latest shareware audit adjustments:
- dog jump entry now uses tic-scaled predictive reach in the same broad shape as `T_DogChase`
- dog jump entry no longer re-checks sight or area connectivity once chase has started, closer to `T_DogChase`
- dog bite resolution no longer re-checks chase visibility/area gating once the jump has started, matching `T_Bite` more closely
- dog bite sound now plays on every bite attempt, including misses
- shareware reaction delays now follow the source split more closely: officer fixed 2 tics, SS `1+Rnd/6`, dog `1+Rnd/8`, guard `1+Rnd/4`, Hans 1 tic
- shareware ranged enemies now have explicit miss-sound regression coverage for guard, officer, SS, and Hans
- guard/officer/SS/Hans chase now separates direct chase from dodge-style chase when line of sight is live, closer to `SelectChaseDir` / `SelectDodgeDir`
- first sighting now carries a one-step first-attack turnaround exemption into dodge selection, closer to `FL_FIRSTATTACK`
- dogs no longer open doors during chase movement
- humanoid enemies now keep closed door goals and wait for the door to open instead of dropping the move immediately
- actor goal movement now follows `MoveObj` more closely for diagonal steps instead of Euclidean-normalized motion
- actor patrol/chase movement now scales by `tics` like the original `move = ob->speed * tics` loops
- actor patrol/chase updates now keep consuming movement after crossing a tile, closer to the source `while (move)` behavior
- actors no longer adopt the far-side area early just because their next goal tile is a door
- actors now keep their current area when stepping onto a door tile, only switching areas after the next non-door walk like `TryWalk`
- one-tile point-blank shot entry now uses remaining tile distance in the same broad shape as `T_Chase`
- ranged hit chance now includes the `T_Shoot` visible-vs-not-visible split instead of treating all line-of-sight shots the same
- guard, officer, and SS pain now choose exactly one source-style 10-tic pain frame based on post-hit HP parity instead of playing both pain frames back-to-back
- guard death now uses the broader `A_DeathScream` scream pool instead of a single fixed sound, and the Episode 1 boss-map special scream override now applies to guard/officer/SS/dog deaths
- the shareware exit tile now starts a BJ run/jump victory sequence instead of ending the level abruptly with no source-style presentation

## Repeatable Enemy Pass

Use the same source-backed process for every enemy pass:
- start from the relevant `WOLFSRC` entry points, not from the current port behavior
- compare spawn mapping, state table coverage, chase logic, attack-start logic, attack resolution, sound triggers, door interaction, death handling, and drop/score behavior
- patch runtime behavior first, then patch the document to record the exact parity detail that landed
- add regression tests for the specific source rule that changed, not just broad smoke tests
- prefer tests that exercise edge cases and deterministic seeds when the original rule depends on RNG
- if a bug is found in one enemy and the same mechanic is shared by other shareware enemies, audit the whole shared family before moving on

When repeating this process for another enemy, explicitly ask:
- does the port use the same chase-vs-dodge decision as `WOLFSRC`
- does the port use the same attack entry conditions and hit chance branches as `WOLFSRC`
- does the port trigger the same sounds on hit, miss, alert, pain, and death
- does the port handle doors, blocking, and movement restrictions the same way
- does the port preserve the same death outcome, pickup drop, and score result

The goal is repeatable parity work:
- not one-off bug fixes
- not "close enough" tuning
- not stopping at the first visible improvement if the shared underlying rule is still wrong

### 4. Sound coverage

Current gap:
- only a subset of enemy-specific sounds exist in the current port

Needed:
- later bosses and non-shareware enemies still need their own sound hooks

### 5. Boss death and progression hooks

Current gap:
- current actor pipeline does not yet model boss-specific progression outcomes

Needed:
- later bosses that require special progression hooks should integrate with level-complete or scripted outcomes when their turn comes

## Practical Milestone Definition

For an enemy to move from `[partial]` or `[next]` to `[done]`, it should have:
- correct map spawn support
- correct animation/state coverage for the current target scope
- correct movement/chase behavior
- correct attack behavior
- correct sounds or clearly wired sound hooks
- correct drop/score/death outcome
- no remaining known material divergence from `WOLFSRC` in the current shareware target scope

Current note:
- the shared enemy AI shell for the shareware Episode 1 roster now has direct source-backed coverage for notice, chase, attack entry, door waiting, area-connectivity, and death/drop behavior

This section is intentionally strict:
- "looks close in playtesting" is not enough
- "same broad AI shell" is not enough
- if a known detail changes gameplay, timing, pathing, or combat outcome, it stays on the checklist until it matches

## Next Concrete Doc Update

When post-shareware work resumes, update this document by:
- moving the next Wolf3D target from `[later]` to `[partial]` as code lands
- keeping the shareware section stable unless a regression is found
