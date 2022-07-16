package tile

import "strings"

type Tile int

func (c Tile) Type() int {
	return int(c) / 10
}

func (c Tile) Number() int {
	return int(c) % 10
}

func (c Tile) String() string {
	return TILE_DATA[c.Type()][c.Number()]
}

func ToTileString(tiles []int) string {
	ret := make([]string, 0, len(tiles))
	for _, t := range tiles {
		ret = append(ret, Tile(t).String())
	}
	return strings.Join(ret, " ")
}
