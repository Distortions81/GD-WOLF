package wl6

const (
	ambushTile = 106
	areaTile   = 107
)

type Direction int

const (
	North Direction = iota
	East
	South
	West
)

type Door struct {
	Vertical bool
	Lock     int
}

type PlayerStart struct {
	X         int
	Y         int
	Direction Direction
}

type Tile struct {
	RawWall uint16
	RawInfo uint16

	Solid      bool
	RenderWall bool
	Area       int
	Ambush     bool
	Door       *Door
}

type Level struct {
	Width        int
	Height       int
	Tiles        []Tile
	PlayerStarts []PlayerStart
}

func (m *MapData) Level() *Level {
	level := &Level{
		Width:  m.Width(),
		Height: m.Height(),
		Tiles:  make([]Tile, m.Width()*m.Height()),
	}

	for y := 0; y < m.Height(); y++ {
		for x := 0; x < m.Width(); x++ {
			rawWall := m.Cell(0, x, y)
			rawInfo := m.Cell(1, x, y)

			tile := Tile{
				RawWall: rawWall,
				RawInfo: rawInfo,
				Area:    -1,
			}

			switch {
			case rawWall == ambushTile:
				// SetupGameLevel clears ambush markers out of the blocking map.
				tile.Ambush = true
			case rawWall >= 90 && rawWall <= 101:
				lock := 0
				vertical := false
				if rawWall%2 == 0 {
					vertical = true
					lock = int((rawWall - 90) / 2)
				} else {
					lock = int((rawWall - 91) / 2)
				}
				tile.Solid = true
				tile.RenderWall = true
				tile.Door = &Door{
					Vertical: vertical,
					Lock:     lock,
				}
			case rawWall < areaTile:
				tile.Solid = true
				tile.RenderWall = true
			default:
				tile.Area = int(rawWall - areaTile)
			}

			if start, ok := playerStartFromInfo(rawInfo); ok {
				level.PlayerStarts = append(level.PlayerStarts, PlayerStart{
					X:         x,
					Y:         y,
					Direction: start,
				})
			}

			level.Tiles[y*level.Width+x] = tile
		}
	}

	return level
}

func (l *Level) Tile(x, y int) Tile {
	if x < 0 || y < 0 || x >= l.Width || y >= l.Height {
		return Tile{}
	}
	return l.Tiles[y*l.Width+x]
}

func playerStartFromInfo(info uint16) (Direction, bool) {
	switch info {
	case 19:
		return North, true
	case 20:
		return East, true
	case 21:
		return South, true
	case 22:
		return West, true
	default:
		return North, false
	}
}
