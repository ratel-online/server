package database

import (
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/server/consts"
)

type Room struct {
	sync.Mutex

	ID                int64     `json:"id"`
	Type              int       `json:"type"`
	Game              *Game     `json:"gameId"`
	UnoGame           *UnoGame  `json:"unoGame"`
	Mahjong           *Mahjong  `json:"mahjong"`
	State             int       `json:"state"`
	Players           int       `json:"players"`
	Robots            int       `json:"robots"`
	Creator           int64     `json:"creator"`
	ActiveTime        time.Time `json:"activeTime"`
	MaxPlayers        int       `json:"maxPlayers"`
	Password          string    `json:"password"`
	EnableChat        bool      `json:"enableChat"`
	EnableLaiZi       bool      `json:"enableLaiZi"`
	EnableSkill       bool      `json:"enableSkill"`
	EnableLandlord    bool      `json:"enableLandlord"`
	EnableDontShuffle bool      `json:"enableDontShuffle"`
	Banker            int64     `json:"banker"`
}

func (r *Room) Model() model.Room {
	return model.Room{
		ID:        r.ID,
		Type:      r.Type,
		TypeDesc:  consts.GameTypes[r.Type],
		Players:   r.Players,
		State:     r.State,
		StateDesc: consts.RoomStates[r.State],
		Creator:   r.Creator,
	}
}

func (room *Room) removePlayer(player *Player) {
	if room == nil || player == nil {
		return
	}
	room.ActiveTime = time.Now()
	playersIds := getRoomPlayers(room.ID)
	if _, ok := playersIds[player.ID]; ok {
		room.Players--
		player.RoomID = 0
		delete(playersIds, player.ID)
		if len(playersIds) > 0 && room.Creator == player.ID {
			for k := range playersIds {
				room.Creator = k
				break
			}
		}
	}
	if len(playersIds) == 0 {
		room.delete()
	}
}

func (room *Room) Cancel() {
	if room.ActiveTime.Add(24 * time.Hour).Before(time.Now()) {
		log.Infof("room %d is timeout 24 hours, removed.\n", room.ID)
		room.delete()
		return
	}
	living := false
	playerIds := getRoomPlayers(room.ID)
	for id := range playerIds {
		if getPlayer(id).online {
			living = true
			break
		}
	}
	if !living {
		log.Infof("room %d is not living, removed.\n", room.ID)
		room.delete()
	}
}

func (room *Room) broadcast(msg string, exclude ...int64) {
	room.ActiveTime = time.Now()
	excludeSet := map[int64]bool{}
	for _, exc := range exclude {
		excludeSet[exc] = true
	}
	roomPlayers := getRoomPlayers(room.ID)
	for playerId := range roomPlayers {
		if player := getPlayer(playerId); player != nil && !excludeSet[playerId] {
			_ = player.WriteString(">> " + msg)
		}
	}
}

func (room *Room) delete() {
	if room != nil {
		rooms.Del(room.ID)
		roomPlayers.Del(room.ID)
		room.Game.delete()
	}
}
