package main

// This opt-in recorder chooses native inputs from map waypoints and observed
// player position. It never changes gameplay state. Its recordings must then
// pass the independent original-source and x86 replay sweep before any parity
// claim can be made.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"gd-wolf/internal/wl6"
)

type demoRouteTile struct{ X, Y int }

func demoMapRoute(level *wl6.Level, start, target demoRouteTile, keys byte) []demoRouteTile {
	queue := []demoRouteTile{start}
	parent := map[demoRouteTile]demoRouteTile{start: start}
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		if at == target {
			var route []demoRouteTile
			for at != start {
				route = append(route, at)
				at = parent[at]
			}
			for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
				route[i], route[j] = route[j], route[i]
			}
			return route
		}
		for _, next := range []demoRouteTile{{at.X - 1, at.Y}, {at.X + 1, at.Y}, {at.X, at.Y - 1}, {at.X, at.Y + 1}} {
			if next.X < 0 || next.Y < 0 || next.X >= level.Width || next.Y >= level.Height {
				continue
			}
			if _, seen := parent[next]; seen {
				continue
			}
			tile := level.Tiles[next.Y*level.Width+next.X]
			if tile.Door != nil {
				if !canOpenDoorLock(tile.Door.Lock, keys) {
					continue
				}
			} else if tile.Solid {
				continue
			}
			parent[next] = at
			queue = append(queue, next)
		}
	}
	return nil
}

func demoRouteInput(g *game, waypoint demoRouteTile, command int, fire bool) wl6.DemoCommand {
	dx, dy := float64(waypoint.X)+0.5-g.playerX, float64(waypoint.Y)+0.5-g.playerY
	fighting := false
	if fire && g.ammo > 0 && g.bestWeapon >= 2 {
		distance := 8.0
		for i := range g.actors {
			actor := &g.actors[i]
			d := math.Hypot(actor.x-g.playerX, actor.y-g.playerY)
			if actor.shootable && !actor.removed && d < distance && g.actorVisibleToPlayer(actor) {
				dx, dy, distance, fighting = actor.x-g.playerX, actor.y-g.playerY, d, true
			}
		}
	}
	target := int(math.Round(math.Atan2(-dy, dx)*180/math.Pi)+360) % 360
	delta := (target-g.demoPlayback.angle+540)%360 - 180
	turn := int(math.Max(-100, math.Min(100, float64(-delta*5))))
	input := wl6.DemoCommand{ControlX: int8(turn)}
	if fighting && absInt(delta) <= 8 {
		input.Buttons |= demoButtonStrafe
		input.ControlX = 70
		if command/24%2 != 0 {
			input.ControlX = -70
		}
		if math.Hypot(dx, dy) > 2.5 {
			input.ControlY = -60
		} else if math.Hypot(dx, dy) < 1.6 {
			input.ControlY = 30
		}
	}
	if absInt(delta) <= 20 && !fighting {
		input.ControlY = int8(-math.Max(1, math.Min(100, math.Round(math.Hypot(dx, dy)*65536/600))))
	}
	// Give USE an unheld edge while stationary at a door. Do not waste the
	// starting ammunition unless a live actor is visible near the crosshair.
	if command%12 < 2 && !g.pushWall.active && !fighting {
		input.Buttons |= demoButtonUse
	}
	if fire && (fighting || command%12 >= 3) {
		for i := range g.actors {
			actor := &g.actors[i]
			if actor.removed || !actor.shootable || !g.actorVisibleToPlayer(actor) {
				continue
			}
			angle := int(math.Round(math.Atan2(g.playerY-actor.y, actor.x-g.playerX)*180/math.Pi)+360) % 360
			if absInt((angle-g.demoPlayback.angle+540)%360-180) <= 12 && math.Hypot(actor.x-g.playerX, actor.y-g.playerY) < 8 {
				input.Buttons |= demoButtonAttack | byte(1<<uint(g.bestWeapon+4))
				break
			}
		}
	}
	return input
}

func demoBossRoute(g *game, target demoRouteTile, fire bool) []demoRouteTile {
	start := demoRouteTile{int(g.playerX), int(g.playerY)}
	// This copy exists only in the planner. Solid decorations obstruct the
	// route just like raw walls; the actual level remains completely unchanged.
	navigation := *g.level
	navigation.Tiles = append([]wl6.Tile(nil), g.level.Tiles...)
	for i := range navigation.Tiles {
		if navigation.Tiles[i].RawInfo == 98 {
			// A planner may request a secret passage. Only an ordinary USE
			// input can actually move its wall in the running game.
			navigation.Tiles[i].Solid = false
		}
	}
	for _, sprite := range g.staticSprites {
		if sprite.alive && sprite.blocking {
			navigation.Tiles[int(sprite.y)*navigation.Width+int(sprite.x)].Solid = true
		}
	}
	if fire {
		var supply []demoRouteTile
		bestRank := 3
		for _, sprite := range g.staticSprites {
			if !sprite.alive {
				continue
			}
			weapon := sprite.pickup == pickupMachineGun && g.bestWeapon < 2 || sprite.pickup == pickupChaingun && g.bestWeapon < 3
			useful := weapon
			useful = useful || g.ammo < 12 && (sprite.pickup == pickupClip || sprite.pickup == pickupClip2)
			useful = useful || g.health < 80 && (sprite.pickup == pickupFood || sprite.pickup == pickupFirstAid || sprite.pickup == pickupFullHeal)
			if !useful {
				continue
			}
			route := demoMapRoute(&navigation, start, demoRouteTile{int(sprite.x), int(sprite.y)}, g.keys)
			rank := 1
			if weapon {
				rank = 0
			}
			if len(route) > 0 && len(route) <= 64 && (rank < bestRank || rank == bestRank && len(route) < len(supply)) {
				supply, bestRank = route, rank
			}
		}
		if len(supply) > 0 {
			return supply
		}
	}
	if route := demoMapRoute(&navigation, start, target, g.keys); len(route) > 0 {
		return route
	}
	// A locked boss room requires an actual key pickup first. Choose the
	// nearest reachable key, then replan after ordinary pickup processing.
	var best []demoRouteTile
	for _, sprite := range g.staticSprites {
		if !sprite.alive || sprite.pickup < pickupKey1 || sprite.pickup > pickupKey4 || g.keys&(1<<uint(sprite.pickup-pickupKey1)) != 0 {
			continue
		}
		route := demoMapRoute(&navigation, start, demoRouteTile{int(sprite.x), int(sprite.y)}, g.keys)
		if len(route) > 0 && (best == nil || len(route) < len(best)) {
			best = route
		}
	}
	return best
}

func TestWolfDemoRecordMapRoutes(t *testing.T) {
	out := os.Getenv("GDWOLF_DEMO_ROUTE_OUT")
	if out == "" {
		t.Skip("set GDWOLF_DEMO_ROUTE_OUT and GDWOLF_DEMO_ROUTE_DATA to generate native boss-room routes")
	}
	out, err := filepath.Abs(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("recording output must be a new directory: %s", out)
	}
	data, err := filepath.Abs(os.Getenv("GDWOLF_DEMO_ROUTE_DATA"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := wl6.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if files.Variant.EpisodeCount != 6 {
		t.Fatal("boss-room corpus requires registered WL6 data")
	}
	if err := os.MkdirAll(filepath.Join(out, "recordings"), 0755); err != nil {
		t.Fatal(err)
	}
	hash := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	assets := map[string]string{}
	for _, name := range []string{"MAPHEAD", "GAMEMAPS", "VSWAP", "VGADICT", "VGAHEAD", "VGAGRAPH", "AUDIOHED", "AUDIOT"} {
		name += ".WL6"
		raw, err := os.ReadFile(filepath.Join(data, name))
		if err != nil {
			t.Fatal(err)
		}
		assets[name] = hash(raw)
	}
	var recordings []map[string]any
	for _, config := range []struct {
		mapIndex int
		kind     ActorKind
		name     string
	}{{8, actorKindBoss, "hans"}, {18, actorKindSchabbs, "schabbs"}, {28, actorKindMecha, "mecha"}, {38, actorKindGift, "gift"}, {48, actorKindGretel, "gretel"}, {58, actorKindFat, "fat"}} {
		for _, fire := range []bool{false, true} {
			g, err := buildEnemyAIFuzzBaseline(files, config.mapIndex)
			if err != nil {
				t.Fatal(err)
			}
			demo := &wl6.Demo{Map: config.mapIndex, Commands: make([]wl6.DemoCommand, 2048)}
			if err := g.startDemo(demo); err != nil {
				t.Fatal(err)
			}
			target := demoRouteTile{-1, -1}
			for _, actor := range g.actors {
				if actor.kind == config.kind {
					target = demoRouteTile{actor.tileX, actor.tileY}
					break
				}
			}
			if target.X < 0 {
				t.Fatalf("map%d has no target %s", config.mapIndex, config.name)
			}
			route := demoBossRoute(g, target, fire)
			if len(route) == 0 {
				t.Fatalf("map%d has no unlocked path to %s", config.mapIndex, config.name)
			}
			waypoint, count, activated := 0, 0, false
			lastAmmo, lastHealth := g.ammo, g.health
			previousX, previousY, stalled := g.playerX, g.playerY, 0
			for count < len(demo.Commands) && !g.playerDying && g.demoPlayback.levelExit == 0 {
				if fire && (g.ammo < 12 && lastAmmo >= 12 || g.health < 30 && lastHealth >= 30) || stalled >= 24 && !g.pushWall.active {
					if next := demoBossRoute(g, target, fire); len(next) > 0 {
						route, waypoint = next, 0
					}
					stalled = 0
				}
				lastAmmo, lastHealth = g.ammo, g.health
				for waypoint < len(route)-1 && math.Hypot(float64(route[waypoint].X)+0.5-g.playerX, float64(route[waypoint].Y)+0.5-g.playerY) < 0.09 {
					waypoint++
				}
				if waypoint == len(route)-1 && route[waypoint] != target && math.Hypot(float64(route[waypoint].X)+0.5-g.playerX, float64(route[waypoint].Y)+0.5-g.playerY) < 0.09 {
					if next := demoBossRoute(g, target, fire); len(next) > 0 {
						route, waypoint = next, 0
					}
				}
				demo.Commands[count] = demoRouteInput(g, route[waypoint], count, fire)
				g.updateDemoPlayback(wl6.DemoTics)
				count++
				if math.Hypot(g.playerX-previousX, g.playerY-previousY) < 0.001 {
					stalled++
				} else {
					stalled = 0
				}
				previousX, previousY = g.playerX, g.playerY
				for _, actor := range g.actors {
					activated = activated || actor.kind == config.kind && actor.demoActive
				}
			}
			raw := make([]byte, 4+3*count)
			raw[0] = byte(config.mapIndex)
			binary.LittleEndian.PutUint16(raw[1:3], uint16(len(raw)))
			for i, command := range demo.Commands[:count] {
				raw[4+i*3], raw[5+i*3], raw[6+i*3] = command.Buttons, byte(command.ControlX), byte(command.ControlY)
			}
			id := fmt.Sprintf("map-%02d-%s-fire%t", config.mapIndex, config.name, fire)
			name := "recordings/" + id + ".wl6"
			if err := os.WriteFile(filepath.Join(out, name), raw, 0644); err != nil {
				t.Fatal(err)
			}
			recordings = append(recordings, map[string]any{"id": id, "file": name, "data": data, "sha256": hash(raw), "data_sha256": assets,
				"source_url": "file://" + filepath.Join(mustDemoRouteWorkingDirectory(t), "wolf_demo_map_route_record_test.go"), "recorded_version": "Generated map-guided native inputs v1; not a human recording or an oracle result",
				"map": config.mapIndex, "pattern": "map-route", "commands": count, "target_kind": config.kind, "target_activated_during_recording": activated,
				"recording_final_health": g.health, "recording_final_ammo": g.ammo, "recording_best_weapon": g.bestWeapon, "recording_final_player_tile": []int{int(g.playerX), int(g.playerY)}, "waypoints_reached": waypoint, "waypoints_total": len(route)})
			t.Logf("%s commands=%d target_active=%v health=%d ammo=%d weapon=%d waypoints=%d/%d", id, count, activated, g.health, g.ammo, g.bestWeapon, waypoint, len(route))
		}
	}
	source, err := os.ReadFile(filepath.Join(mustDemoRouteWorkingDirectory(t), "wolf_demo_map_route_record_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "recorder.go.txt"), source, 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.MarshalIndent(map[string]any{"version": 1, "generator": map[string]any{"source": "recorder.go.txt", "source_sha256": hash(source), "scope": "Ordinary native controls from unchanged hard-difficulty spawns, health, inventory and RNG. Map and observed player position guide inputs. Independent replay is required."}, "recordings": recordings}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "corpus.json"), append(manifest, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustDemoRouteWorkingDirectory(t *testing.T) string {
	t.Helper()
	path, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return path
}
