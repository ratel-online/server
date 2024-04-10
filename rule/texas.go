package rule

type _texasRule struct {
}

func (r _texasRule) Value(key int) int {
	if key == 1 {
		return 13
	}
	return key - 1
}

func (r _texasRule) IsStraight(faces []int, count int) bool {
	// todo
	return false
}

func (r _texasRule) StraightBoundary() (int, int) {
	return 1, 13
}

func (r _texasRule) Reserved() bool {
	return false
}
