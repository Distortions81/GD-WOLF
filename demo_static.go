package main

const demoMaxStatics = 400

const demoDOSVisibilityBase = 0x9c48

type demoStaticInfo struct {
	flags, item byte
	visibility  uint16
}

func newDemoStaticInfo(sprite staticSprite) demoStaticInfo {
	info := demoStaticInfo{item: wolfStaticItemNumber(sprite.pickup), visibility: uint16(demoDOSVisibilityBase + int(sprite.x)*64 + int(sprite.y))}
	if sprite.pickup != pickupNone {
		info.flags = 2
	}
	return info
}

func (g *game) initializeDemoStaticSlots() {
	d := g.demoPlayback
	if d == nil {
		return
	}
	if d.staticSlots != nil {
		for len(d.staticInfo) < len(d.staticSlots) {
			d.staticInfo = append(d.staticInfo, newDemoStaticInfo(g.staticSprites[d.staticSlots[len(d.staticInfo)]]))
		}
		return
	}
	if g.level != nil {
		for y := 0; y < g.levelHeight; y++ {
			for x := 0; x < g.levelWidth; x++ {
				info := g.level.Tile(x, y).RawInfo
				if info == 73 || info == 74 {
					panic("unsupported original memory access: map static type exceeds the non-Spear statinfo table")
				}
			}
		}
	}
	d.staticSlots = make([]int, 0, len(g.staticSprites))
	for i, sprite := range g.staticSprites {
		// The port renders dead-guard map content as a static sprite, but
		// original SpawnDeadGuard allocates an actor and no statobjlist slot.
		if g.level != nil && g.level.Tile(int(sprite.x), int(sprite.y)).RawInfo == 124 && sprite.pickup == pickupNone {
			continue
		}
		d.staticSlots = append(d.staticSlots, i)
		d.staticInfo = append(d.staticInfo, newDemoStaticInfo(sprite))
	}
	if len(d.staticSlots) >= demoMaxStatics {
		panic("original static pool exhausted during SpawnStatic: MAXSTATS=400")
	}
}

// PlaceItemType reuses the first collected item's slot. Its ordering affects
// both same-frame pickups and the original DOS static-object memory layout.
func (g *game) placeDemoStaticItem(sprite staticSprite) {
	g.initializeDemoStaticSlots()
	d := g.demoPlayback
	for slot, index := range d.staticSlots {
		if !g.staticSprites[index].alive {
			g.staticSprites[index] = sprite
			d.staticInfo[slot] = newDemoStaticInfo(sprite)
			return
		}
	}
	if len(d.staticSlots) == demoMaxStatics {
		return
	}
	d.staticSlots = append(d.staticSlots, len(g.staticSprites))
	d.staticInfo = append(d.staticInfo, newDemoStaticInfo(sprite))
	g.staticSprites = append(g.staticSprites, sprite)
}

func wolfPickupForItem(item byte) pickupType {
	items := [...]pickupType{pickupNone, pickupNone, pickupGibs, pickupAlpo, pickupFirstAid,
		pickupKey1, pickupKey2, pickupKey3, pickupKey4, pickupCross, pickupChalice, pickupBible,
		pickupCrown, pickupClip, pickupClip2, pickupMachineGun, pickupChaingun, pickupFood, pickupFullHeal}
	if int(item) < len(items) {
		return items[item]
	}
	return pickupNone
}

func (g *game) collectDemoPickups() {
	g.initializeDemoStaticSlots()
	d := g.demoPlayback
	for slot, index := range d.staticSlots {
		sprite, info := &g.staticSprites[index], d.staticInfo[slot]
		if !sprite.alive || info.flags&2 == 0 {
			continue
		}
		visibility := int(info.visibility) - demoDOSVisibilityBase
		if visibility < 0 || visibility >= 4096 {
			panic("unsupported original memory access: static visibility pointer outside verified spotvis")
		}
		visibleIndex := visibility%64*g.levelWidth + visibility/64
		if visibleIndex >= len(d.visibleTiles) || !d.visibleTiles[visibleIndex] || !g.demoPickupTransformInReach(int(sprite.x), int(sprite.y)) {
			continue
		}
		pickup := wolfPickupForItem(info.item)
		if info.item == 20 {
			panic("unsupported original memory access: DOS alias creates an unverified Spear pickup")
		}
		if pickup != pickupNone {
			if !g.applyPickup(pickup) {
				continue
			}
			g.setNotice("Picked up " + pickupLabel(pickup))
		} else {
			// GetBonus has no default case: unknown item IDs still reach
			// StartBonusFlash and removal after the switch.
			g.startBonusFlash()
		}
		sprite.alive, sprite.blocking = false, false
	}
}

func wolfStaticItemNumber(pickup pickupType) byte {
	switch pickup {
	case pickupGibs:
		return 2
	case pickupAlpo:
		return 3
	case pickupFirstAid:
		return 4
	case pickupKey1:
		return 5
	case pickupKey2:
		return 6
	case pickupKey3:
		return 7
	case pickupKey4:
		return 8
	case pickupCross:
		return 9
	case pickupChalice:
		return 10
	case pickupBible:
		return 11
	case pickupCrown:
		return 12
	case pickupClip:
		return 13
	case pickupClip2:
		return 14
	case pickupMachineGun:
		return 15
	case pickupChaingun:
		return 16
	case pickupFood:
		return 17
	case pickupFullHeal:
		return 18
	default:
		return 0
	}
}
