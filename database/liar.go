package database

import (
	"sync"

	"github.com/ratel-online/core/model"
)

type Liar struct {
	sync.Mutex
	Room         *Room                  `json:"room"`
	PlayerIDs    []int64                `json:"playerIds"`
	Bullets      map[int64]int          `json:"bullets"`
	Bong         map[int64]int          `json:"bong"`
	Pokers       model.Pokers           `json:"pokers"`
	States       map[int64]chan int     `json:"states"`
	Hands        map[int64]model.Pokers `json:"hands"`
	Target       *model.Poker           `json:"target"`
	Alive        map[int64]bool         `json:"alive"`
	LastPlayerID int64                  `json:"lastPlayerId"`
	LastPokers   model.Pokers           `json:"lastPokers"`
	Supervisors  map[int64]bool         `json:"supervisors"`
}

func (l *Liar) Clean() {
}
