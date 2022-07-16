package win

import (
	"sort"

	"github.com/ratel-online/server/mahjong/card"
	"github.com/ratel-online/server/mahjong/util"
)

// CanWin 判断当前牌型是否是胡牌牌型(7对或4A+2B)
// 需要根据手牌和明牌去判断是否胡牌
func CanWin(handTiles, showTiles []int) bool {
	var sortedTiles = util.SliceCopy(handTiles)
	// 升序排列
	sort.Ints(sortedTiles)
	// 找到所有的对
	var pos = FindPairPos(sortedTiles)
	// 找不到对，无法胡牌
	if len(pos) == 0 {
		return false
	}

	// 7对(目前版本只有手中为7个对才可以胡)
	if len(pos) == 7 {
		return true
	}

	// 地龙
	// 手牌有5对;明牌3张;明牌三张相同;且手牌的孤张与明牌相同
	if len(pos) == 5 &&
		len(showTiles) == 3 &&
		showTiles[0] == showTiles[1] && showTiles[0] == showTiles[2] &&
		util.IntInSlice(showTiles[0], handTiles) {
		return true
	}

	// 遍历所有对，因为胡必须有对
	var lastPairTile int // 上次做为对的牌
	for _, v := range pos {
		// 避免有4张同样手牌时，多判断一次
		if sortedTiles[v] == lastPairTile {
			continue
		} else {
			lastPairTile = sortedTiles[v]
		}
		cards := RemovePair(sortedTiles, v)
		if IsAllSequenceOrTriplet(cards) {
			return true
		}
	}
	return false
}

// FindPairPos 找出所有对牌的位置
// 传入的牌需要是已排序的
func FindPairPos(sortedTiles []int) []int {
	var pos = []int{}
	length := len(sortedTiles) - 1
	for i := 0; i < length; i++ {
		if sortedTiles[i] == sortedTiles[i+1] {
			pos = append(pos, i)
			i++
		}
	}
	return pos
}

// RemovePair 从已排序的牌中，移除一对
func RemovePair(sortedTiles []int, pos int) []int {
	remainTiles := make([]int, 0, len(sortedTiles)-2)
	remainTiles = append(remainTiles, sortedTiles[:pos]...)
	remainTiles = append(remainTiles, sortedTiles[pos+2:]...)
	return remainTiles
}

// IsAllSequenceOrTriplet 是否全部顺或者刻
// 传入的牌需要是已排序的
func IsAllSequenceOrTriplet(sortedTiles []int) bool {
	cardsLen := len(sortedTiles)
	for i := 0; i < cardsLen/3; i++ {
		find := FindAndRemoveTriplet(&sortedTiles)
		if !find {
			find = FindAndRemoveSequence(&sortedTiles)
		}
		if !find {
			return false
		}
	}
	return len(sortedTiles) == 0
}

// FindAndRemoveTriplet 从已排序的[]int中移除排头的刻子
func FindAndRemoveTriplet(sortedTiles *[]int) bool {
	var v = *sortedTiles
	if IsTriplet(v[0], v[1], v[2]) {
		*sortedTiles = append([]int{}, v[3:]...)
		return true
	}
	return false
}

// FindAndRemoveSequence 从已排序的[]int中移除排头的顺子
func FindAndRemoveSequence(sortedTiles *[]int) bool {
	var v = *sortedTiles
	var tmp = make([]int, 0)
	for i := 1; i < len(v); i++ {
		switch {
		case v[i] == v[i-1]:
			tmp = append(tmp, v[i])
		case v[i] == v[i-1]+1:
			if v[i]-v[0] == 2 {
				tmp = append(tmp, v[i+1:]...)
				*sortedTiles = tmp
				return true
			}
		default:
			return false
		}
	}
	return false
}

// IsSequence 是否顺子
// 传入的牌必须是已排序的
// 非万、筒、条肯定不是顺
func IsSequence(tileA, tileB, tileC int) bool {
	if !card.IsSuit(tileA) || !card.IsSuit(tileB) || !card.IsSuit(tileC) {
		return false
	}
	if tileB == tileA+1 && tileC == tileB+1 {
		return true
	}
	return false
}

// IsTriplet 是否刻子
func IsTriplet(tileA, tileB, tileC int) bool {
	if tileB == tileA && tileC == tileB {
		return true
	}
	return false
}

// FindSequenceOrTripletCnt 找出当前牌中所有刻和顺的数量
// 返回数量和抽完剩余的牌
func FindSequenceOrTripletCnt(sortedTiles []int) (int, []int) {
	var cnt = 0
	var remain = []int{}
	for {
		if len(sortedTiles) <= 2 {
			remain = append(remain, sortedTiles...)
			break
		}
		find := FindAndRemoveTriplet(&sortedTiles)
		if !find {
			find = FindAndRemoveSequence(&sortedTiles)
		}
		if find {
			cnt++
		} else {
			remain = append(remain, sortedTiles[0])
			sortedTiles = sortedTiles[1:]
		}
	}
	return cnt, remain
}
