# Enemy Parity Checklist

This document tracks enemy parity work against `WOLFSRC`, including all six
Wolf3D episodes. Implemented support and bounded comparison coverage do not
establish complete gameplay parity.

Source references:
- Spawn roster and map object IDs: `WOLFSRC/WL_GAME.C`
- Enemy enums and spawn entry points: `WOLFSRC/WL_DEF.H`
- Core actor logic for regular enemies and bosses: `WOLFSRC/WL_ACT2.C`
- Current port actor implementation: `actor_ai.go`
- Current port actor definitions and sequences: `sprite_catalog.go`
- Registered bosses and projectiles: `registered_actor.go`, `registered_actor_catalog.go`, `actor_projectile.go`

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

## Progress Legend

- `[done]` source-backed baseline with no known material gameplay divergence in the current target scope
- `[partial]` implementation exists, but gameplay parity or its verification is incomplete
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
- `[partial]` Guard — tested movement decisions match; complete runtime parity unverified
- `[partial]` Dog — tested movement decisions match; complete runtime parity unverified
- `[partial]` Officer — tested movement decisions match; complete runtime parity unverified
- `[partial]` SS — tested movement decisions match; complete runtime parity unverified
- `[partial]` Hans Grosse — tested movement decisions match; complete runtime parity unverified
- `[partial]` Mutant
- `[partial]` Gretel Grosse — registered encounter and synthetic chase match original C; complete runtime parity unverified
- `[done]` Dead guard corpse content

Registered gameplay support:
- `[partial]` Dr. Schabbs, Giftmacher and Fatface — chase, projectile attacks, death and death cameras
- `[partial]` Fake Hitler — dodge movement, flame attacks and death
- `[partial]` Mecha-Hitler / Hitler — two-stage transformation and death camera
- `[partial]` Ghosts: Blinky, Clyde, Pinky, Inky — original movement and contact damage

All spawnable families have original-C encounter coverage. Fourteen additional
registered scenarios exercise movement, damage, projectiles, transformation
and death in all three sound modes. The demo runtime also checks original actor
pool slots and tile occupancy when `GDWOLF_DEMO_OCCUPANCY=1` is set. Broader
generated routes remain necessary to find interactions absent from those
encounters; see [the runtime coverage](docs/wolf-demo-compare.md).

## Shareware Episode 1 Track

This is the working order for the document and implementation plan.

The 2026-10-03 [compiled original C audit](docs/wolf-source-compare.md) found
shared movement discrepancies in chase/run fallback ordering, failed-move
direction reset, locked-door decisions and `TryWalk(nodir)` rejection. The first
fix pass corrected fallback ordering, direction reset and `nodir` rejection.
The second pass corrected humanoid locked-door handling, matched all 815
saved failures, and passed all 894,444 original C movement comparisons. These
shared gaps are now closed within the tested movement scope. The compiled
comparison covers isolated movement decisions with actor classes normalized
to humanoid or dog; full original-engine runtime and demo comparison remain
outside its scope. Completed implementation items below do not establish
complete enemy AI parity. Sight/hearing, state timing, combat and interactions
between systems still need independent original-engine comparison.

The [demo comparison](docs/wolf-demo-compare.md) now replays recorded
inputs through the actual port and records actor, door, RNG and combat state.
The player C reference compares conditional movement. A separate C startup
reference verifies actor initialization on all four demo maps and conditional
face RNG updates. The independent runtime reference now matches all four
built-in demo playbacks through their original death endpoints.

### 1. Guard

Shareware Episode 1 relevance:
- Core baseline enemy
- Present from the beginning

Current status:
- `[done]` Spawned and updated in gameplay
- `[done]` Stand, patrol, chase, pain, shoot, and death flow exist in the port with source-backed shareware behavior coverage

Remaining parity gaps:
- no remaining differences in the tested movement decisions
- complete runtime parity remains unverified against the original engine

Notes:
- Guard remains the reference baseline for later humanoid enemy work
- Do not consider it "finished forever"; use it as the standard to compare officer and SS behavior against
- Keep revisiting guard when a source-level mismatch is found elsewhere; if the baseline is wrong, later parity claims are weak

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
- no remaining differences in the tested movement decisions
- complete runtime parity remains unverified against the original engine

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
- no remaining differences in the tested movement decisions
- complete runtime parity remains unverified against the original engine

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
- no remaining differences in the tested movement decisions
- complete runtime parity remains unverified against the original engine

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
- no remaining differences in the tested movement decisions
- complete runtime parity remains unverified against the original engine

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

1. `[done]` Match original humanoid locked-door behavior and recheck
2. `[partial]` Guard runtime parity verification
3. `[partial]` Dog runtime parity verification
4. `[partial]` Officer runtime parity verification
5. `[partial]` SS runtime parity verification
6. `[partial]` Hans Grosse runtime parity verification

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
- `[partial]` all four ghosts use original chase states, movement and contact damage
- Original-C synthetic encounters cover each ghost; original map spawns with an
  out-of-range area byte require separate DOS memory validation

### Later Wolf3D Bosses

Each family is implemented and has bounded original-C runtime checks.

#### Gretel Grosse
- `[partial]` active gameplay support for map object `197`, north-facing ambush spawn, difficulty HP, non-rotating stand/chase/shoot/death sequences, and six-shot bursts
- `[partial]` attack accuracy and firing sound follow the ordinary guard branch of `T_Shoot`; Gretel does not receive Hans's accuracy adjustment. Alert/death sounds use `KEINSND` / `MEINSND`; death drops the gold key and awards 5,000 points
- [Source-derived Go regressions](gretel_test.go) cover spawn HP (950/1,050/1,200), burst exit-action timing, accuracy, no-pain damage response, death timing, sounds, key drop and score. Death/drop behavior has not yet been checked through an independent C kill encounter
- [Original-C enemy comparisons](wolf_enemy_runtime_compare_test.go) cover thirteen spawnable families with registered data. Gretel's E5M9 encounter matches for 19 commands through player death; a separate [synthetic open room](gretel_runtime_test.go) matches for 95 commands and requires both chase movement and damage. These two Gretel probes use hard difficulty and sound off, comparing actor state, RNG, damage and doors; they do not establish complete interactive, rendering or audible sound parity

#### Dr. Schabbs
- `[partial]` needle attacks, chase/run selection, death timing and death camera

#### Giftmacher
- `[partial]` rocket attacks, chase/run selection, death timing and death camera

#### Fatface
- `[partial]` rocket plus gun attack sequence, chase and death camera

#### Fake Hitler
- `[partial]` dodge movement, flame sequence and original alert/death sounds

#### Hitler / Mecha-Hitler
- `[partial]` Mecha gun bursts and footsteps, Hitler spawn on death, subsequent
  Hitler attacks and death camera; dynamic actor order is checked against C

## Cross-Cutting Engine Gaps

These checks span the complete Wolf3D roster.

### 1. Actor instantiation and dispatch

Current coverage:
- all Wolf3D families are instantiated and dispatched
- recorded playback uses original slot allocation, reuse and occupancy tags

Remaining verification:
- unusual interactions between dynamic spawns, stale reservations and doors

Episode 1 value:
- this blocker is cleared for the full shareware Episode 1 roster

### 2. Per-enemy sequences

Current coverage:
- registered bosses, ghosts, projectiles and effects have original state sequences
- comparisons check shape, remaining timer, actions and frozen zero-tic spawns

Remaining verification:
- encounters and timings not reached by the current runtime corpus

Episode 1 value:
- this blocker is cleared for the shareware Episode 1 roster

### 3. Per-enemy combat logic

Current coverage:
- gun, knife, contact, needle, rocket and flame damage paths have source-backed checks

Remaining verification:
- broaden recorded routes and collision boundary cases across registered maps

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
- demo guard deaths now select the original two-sound shareware pool or eight-sound registered pool. The special map-9 scream override is registered-only; mutant deaths use `AHHHGSND`
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

Current coverage:
- all Wolf3D enemy families have source sound IDs and deterministic channel timing
- registered encounter probes pass with effects off, synthesized effects and mixed digitized routing

Remaining verification:
- physical DOS interrupt timing, audible output and mixing

### 5. Boss death and progression hooks

Current coverage:
- recorded playback includes original key drops, Mecha transformation, boss
  death cameras and BJ victory; player/actor state and terminal results are compared

Remaining verification:
- display fades, acknowledgement waits, audio presentation and interactive progression

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
- the shared enemy AI shell has Go regressions based on source behavior for notice, chase, attack entry, door waiting, area-connectivity, and death/drop behavior
- independent compiled C coverage verifies isolated movement decisions, actor initialization on all four built-in demo maps, and the played commands of all four demos through their matching original deaths; original x86 floor visibility matches all four, while shipped-DOS health/ammo has been checked for the first demo only. The complete enemy roster remains unverified

This section is intentionally strict:
- "looks close in playtesting" is not enough
- "same broad AI shell" is not enough
- if a known detail changes gameplay, timing, pathing, or combat outcome, it stays on the checklist until it matches

## Next Concrete Doc Update

When post-shareware work resumes, update this document by:
- moving the next Wolf3D target from `[later]` to `[partial]` as code lands
- expanding independent original-engine coverage and updating the shareware verification status as evidence lands
