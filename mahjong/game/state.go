package game

import (
	"fmt"
	"strings"

	"github.com/ratel-online/server/mahjong/tile"
)

type State struct {
	LastPlayedTile    int
	PlayedTiles       []int
	CurrentPlayerHand []int
	PlayerSequence    []string
}

func (s State) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("playedTiles:%s", tile.ToTileString(s.PlayedTiles)))
	lines = append(lines, fmt.Sprintf("Last played tile: %s", tile.Tile(s.LastPlayedTile).String()))

	var playerStatuses []string
	for _, playerName := range s.PlayerSequence {
		playerStatus := playerName
		playerStatuses = append(playerStatuses, playerStatus)
	}
	lines = append(lines, fmt.Sprintf("Turn order: %s", strings.Join(playerStatuses, ", ")))
	lines = append(lines, fmt.Sprintf("Your hand: %s\n", tile.ToTileString(s.CurrentPlayerHand)))
	return strings.Join(lines, "\n")
}
