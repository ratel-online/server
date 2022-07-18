package game

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ratel-online/server/mahjong/tile"
	"github.com/ratel-online/server/mahjong/ting"
)

type State struct {
	LastPlayer        *playerController
	OriginallyPlayer  *playerController
	CurrentPlayer     *playerController
	LastPlayedTile    int
	PlayedTiles       []int
	CurrentPlayerHand []int
	PlayerSequence    []*playerController
	PlayerShowCards   map[string][]*ShowCard
	SpecialPrivileges map[int64][]int
	CanWin            []*playerController
}

func (s State) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("playedTiles:%s", tile.ToTileString(s.PlayedTiles)))
	var playerStatuses []string

	for _, player := range s.PlayerSequence {
		if player.ID() == s.LastPlayer.ID() {
			continue
		}
		playerStatus := player.Name()
		canTing, _ := ting.CanTing(player.Hand(), player.GetShowCardTiles())
		if canTing {
			playerStatus += "(听)"
		}
		if showCards, ok := s.PlayerShowCards[player.Name()]; ok && len(showCards) > 0 {
			playerStatus += "ShowCards："
			for _, showCard := range showCards {
				playerStatus += fmt.Sprintf("%s ", showCard.String())
			}
		}
		playerStatuses = append(playerStatuses, playerStatus)
	}
	drew := s.CurrentPlayerHand[len(s.CurrentPlayerHand)-1]
	sort.Ints(s.CurrentPlayerHand)
	lines = append(lines, fmt.Sprintf("Turn order:\n %s ", strings.Join(playerStatuses, "\n")))
	lines = append(lines, fmt.Sprintf("%s Last played tile: %s", s.LastPlayer.Name(), tile.Tile(s.LastPlayedTile).String()))
	lines = append(lines, fmt.Sprintf("Your drew: %s ", tile.Tile(drew)))
	if canTing, tingCards := ting.CanTing(s.CurrentPlayer.Hand(), s.CurrentPlayer.GetShowCardTiles()); canTing {
		lines = append(lines, fmt.Sprintf("Your ting: %s ", tile.ToTileString(tingCards)))
	}
	lines = append(lines, fmt.Sprintf("Your hand: %s \n", tile.ToTileString(s.CurrentPlayerHand)))
	return strings.Join(lines, "\n")
}
