package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ratel-online/server/mahjong/tile"
)

type State struct {
	LastPlayer        *playerController
	OriginallyPlayer  *playerController
	LastPlayedTile    int
	PlayedTiles       []int
	CurrentPlayerHand []int
	PlayerSequence    []string
	PlayerShowCards   map[string][]*ShowCard
	SpecialPrivileges map[int64][]int
	CanWin            []*playerController
}

func (s State) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("playedTiles:%s", tile.ToTileString(s.PlayedTiles)))
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
	drew := s.CurrentPlayerHand[len(s.CurrentPlayerHand)-1]
	sort.Ints(s.CurrentPlayerHand)
	lines = append(lines, fmt.Sprintf("Turn order:\n%s ", strings.Join(playerStatuses, "\n")))
	lines = append(lines, fmt.Sprintf("Last played tile: %s", tile.Tile(s.LastPlayedTile).String()))
	lines = append(lines, fmt.Sprintf("Your drew: %s ", tile.Tile(drew)))
	lines = append(lines, fmt.Sprintf("Your hand: %s \n", tile.ToTileString(s.CurrentPlayerHand)))
	return strings.Join(lines, "\n")
}
