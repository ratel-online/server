package database

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/feel-easy/mahjong/card"
	"github.com/feel-easy/mahjong/consts"
	"github.com/feel-easy/mahjong/event"
	"github.com/feel-easy/mahjong/game"
	"github.com/feel-easy/mahjong/tile"
	rconsts "github.com/ratel-online/server/consts"
)

type Mahjong struct {
	Room      *Room            `json:"room"`
	PlayerIDs []int            `json:"playerIds"`
	States    map[int]chan int `json:"states"`
	Game      *game.Game       `json:"game"`
}

func (game *Mahjong) Clean() {
	if game != nil {
		for _, state := range game.States {
			close(state)
		}
	}
}

type OP struct {
	operation int
	tiles     []int
}

func circled(n int) string {
	if n >= 1 && n <= 20 {
		return string(rune(0x2460 + n - 1))
	}
	return strconv.Itoa(n)
}

type MahjongPlayer struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func NewPlayer(user *Player) *MahjongPlayer {
	return &MahjongPlayer{
		ID:   user.ID,
		Name: user.Name,
	}
}

func (p *MahjongPlayer) PlayerID() int {
	return int(p.ID)
}

func (p *MahjongPlayer) NickName() string {
	return p.Name
}

func (mp *MahjongPlayer) OnPlayTile(payload event.PlayTilePayload) {
	p := GetPlayer(mp.ID)
	p.WriteString(fmt.Sprintf("You play %s ! \n", tile.Tile(payload.Tile)))
	Broadcast(p.RoomID, fmt.Sprintf("%s PlayTile %s !\n", payload.PlayerName, tile.Tile(payload.Tile)), p.ID)
}

func (mp *MahjongPlayer) Take(tiles []int, gameState game.State) (int, []int, error) {
	p := GetPlayer(mp.ID)
	Broadcast(p.RoomID, fmt.Sprintf("It's %s take mahjong! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your take mahjong, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	askBuf := bytes.Buffer{}
	tileOptions := make(map[string]*OP)
	labelCounter := 1
	if pvs, ok := gameState.SpecialPrivileges[int(p.ID)]; ok {
		for _, pv := range pvs {
			switch pv {
			case consts.GANG:
				askBuf.WriteString("You can 杠!!!\n")
				label := strconv.Itoa(labelCounter)
				ts := []int{gameState.LastPlayedTile, gameState.LastPlayedTile, gameState.LastPlayedTile}
				tileOptions[label] = &OP{
					operation: consts.GANG,
					tiles:     append(ts, gameState.LastPlayedTile),
				}
				askBuf.WriteString(fmt.Sprintf("%s. %s \n", circled(labelCounter), tile.ToTileString(ts)))
				labelCounter++
			case consts.PENG:
				askBuf.WriteString("You can 碰!!!\n")
				label := strconv.Itoa(labelCounter)
				ts := []int{gameState.LastPlayedTile, gameState.LastPlayedTile}
				tileOptions[label] = &OP{
					operation: consts.PENG,
					tiles:     append(ts, gameState.LastPlayedTile),
				}
				askBuf.WriteString(fmt.Sprintf("%s. %s \n", circled(labelCounter), tile.ToTileString(ts)))
				labelCounter++
			case consts.CHI:
				askBuf.WriteString("You can 吃!!!\n")
				for _, ts := range card.CanChiTiles(tiles, gameState.LastPlayedTile) {
					label := strconv.Itoa(labelCounter)
					tileOptions[label] = &OP{
						operation: consts.CHI,
						tiles:     append(ts, gameState.LastPlayedTile),
					}
					askBuf.WriteString(fmt.Sprintf("%s. %s \n", circled(labelCounter), tile.ToTileString(ts)))
					labelCounter++
				}
			}
		}
	}
	label := strconv.Itoa(labelCounter)
	askBuf.WriteString(fmt.Sprintf("%s. %s \n", circled(labelCounter), "no"))
	tileOptions[label] = &OP{
		operation: 0,
		tiles:     []int{},
	}
	for {
		p = getPlayer(p.ID)
		p.WriteString(askBuf.String())
		selectedLabel, err := p.AskForString(consts.PlayMahjongTimeout)
		if err != nil {
			switch err {
			case rconsts.ErrorsExist:
				p.WriteString("Don't quit a good game！\n")
				selectedLabel = "E"
			case rconsts.ErrorsTimeout:
				selectedLabel = "1"
			default:
				return 0, nil, err
			}
		}
		selected, found := tileOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		return selected.operation, selected.tiles, nil
	}
}

func (mp *MahjongPlayer) Play(tiles []int, gameState game.State) (int, error) {
	p := GetPlayer(mp.ID)
	Broadcast(p.RoomID, fmt.Sprintf("It's %s turn! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your turn, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	askBuf := bytes.Buffer{}
	askBuf.WriteString("Select a tile to play:\n")
	tileOptions := make(map[string]int)
	sort.Ints(tiles)
	for idx, i := range tiles {
		label := strconv.Itoa(idx + 1)
		tileOptions[label] = i
		askBuf.WriteString(fmt.Sprintf("%s. %-6s", circled(idx+1), tile.Tile(i).String()))
		if (idx+1)%6 == 0 {
			askBuf.WriteString("\n")
		} else {
			askBuf.WriteString("  ")
		}
	}
	askBuf.WriteString("\n")
	for {
		p = GetPlayer(p.ID)
		p.WriteString(askBuf.String())
		selectedLabel, err := p.AskForString(rconsts.PlayMahjongTimeout)
		if err != nil {
			switch err {
			case rconsts.ErrorsExist:
				p.WriteString("Don't quit a good game！\n")
				selectedLabel = "E"
			case rconsts.ErrorsTimeout:
				selectedLabel = "1"
			default:
				return 0, err
			}
		}
		selectedCard, found := tileOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		mp.OnPlayTile(event.PlayTilePayload{
			PlayerName: p.Name,
			Tile:       selectedCard,
		})
		return selectedCard, nil
	}
}
