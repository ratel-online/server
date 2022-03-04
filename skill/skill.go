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
	consts.Skill996:  N996Skill{},
	consts.SkillTZJW: TZJWSkill{},
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
	return fmt.Sprintf("%s 触发技能<我要色色>，其余玩家沉迷其中，趁机偷掉了他们的最牛的牌", player.Name)
}

func (WYSSSkill) Apply(player *database.Player, game *database.Game) {
	buf := bytes.Buffer{}
	for _, id := range game.Players {
		if id == player.ID {
			continue
		}
		l := len(game.Pokers[id])
		if l > 1 {
			max := game.Pokers[id][l-1]
			game.Pokers[id] = game.Pokers[id][:l-1]
			game.Pokers[player.ID] = append(game.Pokers[player.ID], max)
			game.Pokers[player.ID].SortByOaaValue()
			buf.WriteString(fmt.Sprintf("%s 偷掉了 %s 的牌 %s\n", player.Name, database.GetPlayer(id).Name, model.Pokers{max}.OaaString()))
		}
	}
	database.Broadcast(player.RoomID, buf.String())
}

type HYJJSkill struct{}

func (HYJJSkill) Name() string {
	return "火眼金睛"
}

func (HYJJSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<火眼金睛>，看穿了对手的牌", player.Name)
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
	return fmt.Sprintf("%s 触发技能<改换家门>，手牌重新分配", player.Name)
}

func (GHJMSkill) Apply(player *database.Player, game *database.Game) {
	l := len(game.Pokers[player.ID])
	keys := poker.RandomN(l)
	pokers := poker.GetPokers(keys...)
	for i := range pokers {
		pokers[i].Val = game.Rules.Value(pokers[i].Key)
	}
	if game.Room.EnableLaiZi {
		pokers.SetOaa(game.Universals...)
		pokers.SortByOaaValue()
	}
	game.Pokers[player.ID] = pokers
}

type PFCZSkill struct{}

func (PFCZSkill) Name() string {
	return "破斧沉舟"
}

func (PFCZSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<破斧沉舟>，只留下5张最强的牌", player.Name)
}

func (PFCZSkill) Apply(player *database.Player, game *database.Game) {
	pokers := game.Pokers[player.ID]
	pokers.SortByValue()
	l := len(pokers)
	if l > 5 {
		game.Pokers[player.ID] = pokers[l-5:]
	}
	game.Pokers[player.ID].SortByOaaValue()
}

type DHXJSkill struct{}

func (DHXJSkill) Name() string {
	return "大幻想家"
}

func (DHXJSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<大幻想家>，最小的一张牌变成了癞子", player.Name)
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
	return fmt.Sprintf("%s 触发技能<两极反转>，随机与一名玩家调换手牌", player.Name)
}

func (LJFZSkill) Apply(player *database.Player, game *database.Game) {
	var targetPlayerId int64 = 0
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for targetPlayerId == int64(0) {
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
	return fmt.Sprintf("%s 触发技能<追亡逐北>，多获得一次出牌机会", player.Name)
}

func (ZWZBSkill) Apply(player *database.Player, game *database.Game) {
	game.PlayTimes[player.ID] = 2
}

type SKLFSkill struct{}

func (SKLFSkill) Name() string {
	return "时空裂缝"
}

func (SKLFSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<时空裂缝>，其余玩家出牌时间减半", player.Name)
}

func (SKLFSkill) Apply(player *database.Player, game *database.Game) {
	for id := range game.PlayTimeOut {
		if id == player.ID {
			continue
		}
		if game.PlayTimeOut[id] >= 10*time.Second {
			game.PlayTimeOut[id] /= 2
		} else {
			game.PlayTimeOut[id] = 5 * time.Second
		}
	}
}

type N996Skill struct{}

func (N996Skill) Name() string {
	return "996"
}

func (N996Skill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<996>，所有对手强制获得9,9,6三张牌", player.Name)
}

func (N996Skill) Apply(player *database.Player, game *database.Game) {
	pokers := poker.GetPokers(9, 9, 6)
	for i := range pokers {
		pokers[i].Val = game.Rules.Value(pokers[i].Key)
	}
	if game.Room.EnableLaiZi {
		pokers.SetOaa(game.Universals...)
	}
	for _, id := range game.Players {
		if id == player.ID {
			continue
		}
		game.Pokers[id] = append(game.Pokers[id], pokers...)
		game.Pokers[id].SortByOaaValue()
	}
}

type TZJWSkill struct{}

func (TZJWSkill) Name() string {
	return "添砖加瓦"
}

func (TZJWSkill) Desc(player *database.Player) string {
	return fmt.Sprintf("%s 触发技能<添砖加瓦>，从弃牌池中随机抽取两张牌返还给所有对手", player.Name)
}

func (TZJWSkill) Apply(player *database.Player, game *database.Game) {
	buf := bytes.Buffer{}
	pks := model.Pokers{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	l := len(game.Discards)
	for i := 0; i < Min(2, l); i++ {
		target := r.Intn(len(game.Discards))
		pks = append(pks, game.Discards[target])
		game.Discards = append(game.Discards[:target], game.Discards[target+1:]...)
	}
	if len(pks) > 0 {
		for _, id := range game.Players {
			if id == player.ID {
				continue
			}
			game.Pokers[id] = append(game.Pokers[id], pks...)
			game.Pokers[id].SortByOaaValue()
			buf.WriteString(database.GetPlayer(id).Name + ",")
		}
		buf.Truncate(buf.Len() - 1)
		buf.WriteString("获得了" + pks.OaaString())
		buf.WriteString("\n")
		database.Broadcast(player.RoomID, buf.String())
	}
}

func Min(i, j int) int {
	if i < j {
		return i
	}
	return j
}
