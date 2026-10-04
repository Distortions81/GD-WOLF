package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"gd-wolf/internal/wl6"
)

// TestWolfEnemyBehaviorCompare samples enemy situations on every selected
// registered map. The reference decisions in enemy_ai_harness_test.go are an
// independent Go translation of WOLFSRC; recorded-demo comparisons use the
// compiled original C separately.
func TestWolfEnemyBehaviorCompare(t *testing.T) {
	dataDir := os.Getenv("GDWOLF_ENEMY_COMPARE_DATA")
	if dataDir == "" {
		t.Skip("run scripts/wolf_enemy_behavior_compare.sh --data <registered WL6 directory>")
	}
	files, err := wl6.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	maps, err := files.Maps()
	if err != nil {
		t.Fatal(err)
	}
	selected := -1
	if raw := os.Getenv("GDWOLF_ENEMY_COMPARE_MAP_INDEX"); raw != "" {
		selected, err = strconv.Atoi(raw)
		if err != nil || selected < 0 || selected >= files.Variant.EpisodeCount*10 {
			t.Fatalf("invalid map index %q", raw)
		}
	}
	reportPath := strings.TrimSpace(os.Getenv("GDWOLF_ENEMY_COMPARE_REPORT"))
	report, err := openEnemyAIHarnessReport(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := report.Close(); err != nil {
			t.Error(err)
		}
	}()
	mapCount, actorCount, placementCount, checks, mismatches := 0, 0, 0, 0, 0
	for _, summary := range maps {
		if selected >= 0 && summary.Index != selected {
			continue
		}
		base, err := buildEnemyAIFuzzBaseline(files, summary.Index)
		if err != nil {
			t.Fatalf("map %d: %v", summary.Index, err)
		}
		mapCount++
		for _, actorIndex := range fuzzLiveEnemyIndices(base) {
			actorCount++
			positions := fuzzNearbyPlayerPositions(base, &base.actors[actorIndex])
			if len(positions) == 0 {
				continue
			}
			seen := map[int]bool{}
			for _, pick := range []int{0, len(positions) / 2, len(positions) - 1} {
				if seen[pick] {
					continue
				}
				seen[pick] = true
				pos := positions[pick]
				placementCount++
				for _, mismatch := range []*enemyAIHarnessMismatch{
					compareEnemyAIFirstSightingScenario(base, actorIndex, pos),
					compareEnemyAIAttackEntryScenario(base, actorIndex, pos, 4),
					compareEnemyAIMovementScenario(base, actorIndex, pos),
					compareEnemyAIRuntimeScenario(base, actorIndex, pos, 4),
				} {
					checks++
					if mismatch == nil {
						continue
					}
					mismatches++
					if err := report.Write(*mismatch); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
	if mapCount == 0 {
		t.Fatalf("no maps selected (index %d)", selected)
	}
	t.Logf("enemy behavior comparison: maps=%d actors=%d placements=%d checks=%d mismatches=%d", mapCount, actorCount, placementCount, checks, mismatches)
	if mismatches != 0 {
		t.Fatal(fmt.Sprintf("enemy behavior comparison found %d mismatches; report=%s", mismatches, reportPath))
	}
}
