package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type wolfActorSchedulerState struct {
	Shape    int  `json:"shape"`
	TicCount int  `json:"tic_count"`
	Think    bool `json:"think"`
	Shots    int  `json:"shots"`
	Bites    int  `json:"bites"`
}

func TestWolfActorStatesCompare(t *testing.T) {
	path := os.Getenv("GDWOLF_ACTOR_STATES_REFERENCE")
	if path == "" {
		t.Skip("run scripts/wolf_actor_states_compare.sh")
	}
	sequences := [6][5]AnimSequenceID{
		{seqActorGuardStand, seqActorGuardPatrol, seqActorGuardChase, seqActorGuardShoot, seqActorGuardPain},
		{seqActorOfficerStand, seqActorOfficerPatrol, seqActorOfficerChase, seqActorOfficerShoot, seqActorOfficerPain},
		{seqActorSSStand, seqActorSSPatrol, seqActorSSChase, seqActorSSShoot, seqActorSSPain},
		{"", seqActorDogPatrol, seqActorDogChase, seqActorDogJump, ""},
		{seqActorBossStand, "", seqActorBossChase, seqActorBossShoot, ""},
		{seqActorMutantStand, seqActorMutantPatrol, seqActorMutantChase, seqActorMutantShoot, seqActorMutantPain},
	}
	p := startWolfSourceBinary(t, path)
	for kind, modes := range sequences {
		for mode, id := range modes {
			if id == "" {
				continue
			}
			t.Run(fmt.Sprintf("kind_%d_mode_%d", kind, mode), func(t *testing.T) {
				first, _ := LookupAnimSequence(id)
				for _, remaining := range []int{first.Frames[0].Tics, 0, 1} {
					if remaining > first.Frames[0].Tics {
						continue
					}
					g := &game{}
					a := actorInstance{kind: ActorKind(kind), chaseSeq: modes[2]}
					a.aiState = []ActorAIState{actorStateStand, actorStatePatrol, actorStateChase, actorStateShoot, actorStatePain}[mode]
					if kind == int(actorKindDog) && mode == 3 {
						a.aiState = actorStateJump
					}
					g.startActorSequence(&a, id, first.Loop)
					a.frameTimer = first.Frames[0].Tics - remaining
					a.spawnAnimationFrozen = remaining == 0 && first.Frames[0].Tics > 0
					if _, err := fmt.Fprintf(p.in, "reset %d %d %d\n", kind, mode, remaining); err != nil {
						t.Fatal(err)
					}
					shots, bites := 0, 0
					for step := 0; step < 96; step++ {
						tics := []int{4, 4, 1, 7, 20, 70}[step%6]
						if _, err := fmt.Fprintf(p.in, "step %d\n", tics); err != nil {
							t.Fatal(err)
						}
						if err := p.in.Flush(); err != nil {
							t.Fatal(err)
						}
						if !p.out.Scan() {
							t.Fatal("original scheduler returned no state")
						}
						var want wolfActorSchedulerState
						if err := json.Unmarshal(p.out.Bytes(), &want); err != nil {
							t.Fatal(err)
						}
						think := g.advanceDemoActorSequence(&a, tics, func(action AnimAction) {
							if action == animActionFireActor {
								shots++
							} else if action == animActionBiteActor {
								bites++
							}
						})
						seq, _ := LookupAnimSequence(a.sequenceID)
						count := seq.Frames[a.frameIndex].Tics - a.frameTimer
						if a.spawnAnimationFrozen {
							count = 0
						}
						got := wolfActorSchedulerState{a.shapenum, count, think, shots, bites}
						if got != want {
							t.Fatalf("initial timer %d step %d (%d tics): original=%+v port=%+v", remaining, step, tics, want, got)
						}
					}
				}
			})
		}
	}
}
