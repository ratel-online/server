package rule

var (
	// LandlordRules 斗地主规则
	LandlordRules = _rules{reserved: true}
	// TeamRules 团队规则
	TeamRules = _rules{}
	// RunFastRules 跑得快規則
	RunFastRules = _rules{reserved: true, isRunFast: true}
	// TexasRules 德州扑克规则
	TexasRules = _texasRule{}
)

type _rules struct {
	reserved  bool
	isRunFast bool
}

func (r _rules) Value(key int) int {
	if key == 1 {
		return 12
	} else if key == 2 {
		return 13
	} else if key > 13 {
		return key
	}
	return key - 2
}

func (r _rules) IsStraight(faces []int, count int) bool {
	if faces[len(faces)-1]-faces[0] != len(faces)-1 {
		return false
	}
	if faces[len(faces)-1] > 12 {
		return false
	}
	if count == 1 {
		return len(faces) >= 5
	} else if count == 2 && r.isRunFast {
		return len(faces) >= 2
	} else if count == 2 && !r.isRunFast {
		return len(faces) >= 3
	} else if count > 2 {
		return len(faces) >= 2
	}
	return false
}

func (r _rules) StraightBoundary() (int, int) {
	return 1, 12
}

func (r _rules) Reserved() bool {
	return r.reserved
}
