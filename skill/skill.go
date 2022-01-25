package skill

import (
	"bytes"
	"fmt"
	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
	"math/rand"
	"time"
)

var Skills = map[consts.SkillID]Skill{
	consts.SkillWYSS: WYSSSkill{},
	consts.SkillHYJJ: HYJJSkill{},
	consts.SkillDHXJ: DHXJSkill{},
	consts.SkillGHJM: GHJMSkill{},
	consts.SkillPFCZ: PFCZSkill{},
	consts.SkillLJFZ: LJFZSkill{},
	consts.SkillZWZB: ZWZBSkill{},
	consts.SkillSKLF: SKLFSkill{},
}

type Skill interface {
	Name() string
	Desc(player *database.Player) string
	Apply(player *database.Player, game *database.Game)
}

type WYSSSkill struct{}

func (WYSSSkill) Name() string {
	return "我要色色"
}

func (WYSSSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<我要色色>，其余玩家沉迷其中，趁机偷掉了他们的最牛的牌", player.Name)
}

func (WYSSSkill) Apply(player *database.Player, game *database.Game) {
	buf := bytes.Buffer{}
	for _, id := range game.Players {
		if id == player.ID {
			continue
		}
		l := len(game.Pokers[id])
		max := game.Pokers[id][l-1]
		game.Pokers[id] = game.Pokers[id][:l-1]
		game.Pokers[player.ID] = append(game.Pokers[player.ID], max)
		game.Pokers[player.ID].SortByOaaValue()
		buf.WriteString(fmt.Sprintf("%s 偷掉了 %s 的牌 %s\n", player.Name, database.GetPlayer(id).Name, model.Pokers{max}.OaaString()))
	}
	database.Broadcast(player.RoomID, buf.String())
}

type HYJJSkill struct{}

func (HYJJSkill) Name() string {
	return "火眼金睛"
}

func (HYJJSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<火眼金睛>，看穿了对手的牌", player.Name)
}

func (HYJJSkill) Apply(player *database.Player, game *database.Game) {
	buf := bytes.Buffer{}
	for _, id := range game.Players {
		if id == player.ID {
			continue
		}
		buf.WriteString(fmt.Sprintf("%s: %s\n", database.GetPlayer(id).Name, game.Pokers[id].OaaString()))
	}
	_ = player.WriteString(buf.String())
}

type GHJMSkill struct{}

func (GHJMSkill) Name() string {
	return "改换家门"
}

func (GHJMSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<改换家门>，手牌重新分配", player.Name)
}

func (GHJMSkill) Apply(player *database.Player, game *database.Game) {
	l := len(game.Pokers[player.ID])
	pokersArr, _ := poker.Distribute(1, true, game.Rules)
	pokers := pokersArr[0][:l]
	pokers.SetOaa(game.Universals...)
	pokers.SortByOaaValue()
	game.Pokers[player.ID] = pokers
}

type PFCZSkill struct{}

func (PFCZSkill) Name() string {
	return "破斧沉舟"
}

func (PFCZSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<破斧沉舟>，只留下一张最小的牌和一张最大的牌", player.Name)
}

func (PFCZSkill) Apply(player *database.Player, game *database.Game) {
	pokers := game.Pokers[player.ID]
	pokers.SortByValue()
	l := len(pokers)
	if l > 2 {
		min := pokers[0]
		max := pokers[l-1]
		game.Pokers[player.ID] = model.Pokers{min, max}
		game.Pokers[player.ID].SortByOaaValue()
	}
}

type DHXJSkill struct{}

func (DHXJSkill) Name() string {
	return "大幻想家"
}

func (DHXJSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<大幻想家>，最小的一张牌变成了癞子", player.Name)
}

func (DHXJSkill) Apply(player *database.Player, game *database.Game) {
	pokers := game.Pokers[player.ID]
	pokers[0].Oaa = true
	pokers.SortByOaaValue()
}

type LJFZSkill struct{}

func (LJFZSkill) Name() string {
	return "两极反转"
}

func (LJFZSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<两极反转>，随机与一名玩家调换手牌", player.Name)
}

func (LJFZSkill) Apply(player *database.Player, game *database.Game) {
	var targetPlayerId int64 = 0
	for targetPlayerId == int64(0) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		p := game.Players[r.Intn(len(game.Players))]
		if p != player.ID {
			targetPlayerId = p
		}
	}
	game.Pokers[targetPlayerId], game.Pokers[player.ID] = game.Pokers[player.ID], game.Pokers[targetPlayerId]
}

type ZWZBSkill struct{}

func (ZWZBSkill) Name() string {
	return "追亡逐北"
}

func (ZWZBSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<追亡逐北>，多获得一次出牌机会", player.Name)
}

func (ZWZBSkill) Apply(player *database.Player, game *database.Game) {
	game.PlayTimes[player.ID] = 2
}

type SKLFSkill struct{}

func (SKLFSkill) Name() string {
	return "时空裂缝"
}

func (SKLFSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 使用了技能<时空裂缝>，其余玩家出牌时间缩短5秒", player.Name)
}

func (SKLFSkill) Apply(player *database.Player, game *database.Game) {
	for id := range game.PlayTimeOut {
		if id == player.ID {
			continue
		}
		if game.PlayTimeOut[id] > 5*time.Second {
			game.PlayTimeOut[id] -= 5 * time.Second
		}
	}
}
