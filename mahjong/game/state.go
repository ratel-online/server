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
	PlayerShowCards   map[string][]*ShowCard
	SpecialPrivileges map[int64]int
}

func (s State) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("playedTiles:%s", tile.ToTileString(s.PlayedTiles)))
	lines = append(lines, fmt.Sprintf("Last played tile: %s", tile.Tile(s.LastPlayedTile).String()))
	var playerStatuses []string
	for _, playerName := range s.PlayerSequence {
		playerStatus := playerName
		if showCards, ok := s.PlayerShowCards[playerName]; ok {
			playerStatus += "ShowCards："
			for _, showCard := range showCards {
				playerStatus += fmt.Sprintf("%s ", showCard.String())
			}
		}
		playerStatuses = append(playerStatuses, playerStatus)
	}
	lines = append(lines, fmt.Sprintf("Turn order:\n%s \n", strings.Join(playerStatuses, "\n")))
	lines = append(lines, fmt.Sprintf("Your hand: %s \n", tile.ToTileString(s.CurrentPlayerHand)))
	return strings.Join(lines, "\n")
}
