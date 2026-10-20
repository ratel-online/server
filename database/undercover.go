package database

import (
	"fmt"
	"sync"

	"github.com/lipp12138/chatroom"
	"github.com/ratel-online/core/util/rand"
)

// Undercover 谁是卧底游戏数据模型
type Undercover struct {
	sync.Mutex
	Room            *Room              `json:"room"`
	PlayerIDs       []int64            `json:"playerIds"`       // 玩家ID列表（按发言顺序）
	States          map[int64]chan int `json:"states"`          // 玩家状态通道
	Words           map[int64]string   `json:"words"`           // 每个玩家分配到的词
	IsUndercover    map[int64]bool     `json:"isUndercover"`    // 是否是卧底
	IsBlankWord     map[int64]bool     `json:"isBlankWord"`     // 是否是空白词
	Alive           map[int64]bool     `json:"alive"`           // 是否存活
	PlayerNumbers   map[int64]int      `json:"playerNumbers"`   // 玩家号码牌
	Round           int                `json:"round"`           // 当前轮数
	TurnIndex       int                `json:"turnIndex"`       // 当前发言玩家索引
	Descriptions    map[int64]string   `json:"descriptions"`    // 本轮玩家的描述
	Votes           map[int64]int64    `json:"votes"`           // 投票记录（投票人->被投人）
	NormalWord      string             `json:"normalWord"`      // 平民词
	UndercoverWord  string             `json:"undercoverWord"`  // 卧底词
	IsClockwise     bool               `json:"isClockwise"`     // 是否正序发言
	GameOver        bool               `json:"gameOver"`        // 游戏是否结束
	VoteCounting    bool               `json:"voteCounting"`    // 是否正在计票，防止重复结算
	VoteTargets     []int64            `json:"voteTargets"`     // 当前投票阶段允许被投票的玩家列表；为空表示所有存活玩家
	TiebreakPlayers []int64            `json:"tiebreakPlayers"` // 平票需要补充描述的玩家列表
	RevealUndercoverIDs []int64        `json:"revealUndercoverIds"` // 本轮需要爆词的卧底玩家ID列表
	RevealUsed      map[int64]bool     `json:"revealUsed"`     // 记录卧底是否已经使用过爆词
	RevealWinner    bool               `json:"revealWinner"`   // 爆词环节是否已产生胜者
}

// Clean 清理游戏资源
func (u *Undercover) Clean() {
	if u != nil {
		for _, state := range u.States {
			close(state)
		}
	}
}

// UndercoverWordPair 词组对
type UndercoverWordPair struct {
	NormalWord     string
	UndercoverWord string
}

// WordPairs 预设的词组列表
var WordPairs = []UndercoverWordPair{
	{NormalWord: "苹果", UndercoverWord: "梨子"},
	{NormalWord: "牛奶", UndercoverWord: "豆浆"},
	{NormalWord: "饺子", UndercoverWord: "包子"},
	{NormalWord: "手机", UndercoverWord: "电话"},
	{NormalWord: "可乐", UndercoverWord: "雪碧"},
	{NormalWord: "眼镜", UndercoverWord: "放大镜"},
	{NormalWord: "面条", UndercoverWord: "米线"},
	{NormalWord: "面包", UndercoverWord: "蛋糕"},
	{NormalWord: "篮球", UndercoverWord: "足球"},
	{NormalWord: "钢琴", UndercoverWord: "吉他"},
	{NormalWord: "公交车", UndercoverWord: "地铁"},
	{NormalWord: "医生", UndercoverWord: "护士"},
	{NormalWord: "老师", UndercoverWord: "教授"},
	{NormalWord: "警察", UndercoverWord: "保安"},
	{NormalWord: "咖啡", UndercoverWord: "茶"},
	{NormalWord: "汉堡", UndercoverWord: "三明治"},
	{NormalWord: "电视", UndercoverWord: "电脑"},
	{NormalWord: "冰箱", UndercoverWord: "空调"},
	{NormalWord: "火车", UndercoverWord: "高铁"},
	{NormalWord: "自行车", UndercoverWord: "电动车"},
	{NormalWord: "太阳", UndercoverWord: "月亮"},
	{NormalWord: "大海", UndercoverWord: "湖泊"},
	{NormalWord: "森林", UndercoverWord: "草原"},
	{NormalWord: "玫瑰", UndercoverWord: "康乃馨"},
	{NormalWord: "巧克力", UndercoverWord: "糖果"},
	{NormalWord: "雨伞", UndercoverWord: "雨衣"},
	{NormalWord: "手表", UndercoverWord: "闹钟"},
	{NormalWord: "书包", UndercoverWord: "钱包"},
	{NormalWord: "沙发", UndercoverWord: "椅子"},
	{NormalWord: "窗户", UndercoverWord: "门"},
}

// PickUndercoverWordPair 优先使用 chatroom 词库取词，失败时回退到内置词库。
func PickUndercoverWordPair() (UndercoverWordPair, error) {
	pair, err := chatroom.Pick()
	if err == nil {
		return UndercoverWordPair{
			NormalWord:     pair.Civilian,
			UndercoverWord: pair.Undercover,
		}, nil
	}

	if len(WordPairs) == 0 {
		return UndercoverWordPair{}, fmt.Errorf("pick chatroom word pair: %w", err)
	}

	fallback := WordPairs[rand.Intn(len(WordPairs))]
	return fallback, fmt.Errorf("pick chatroom word pair: %w", err)
}
