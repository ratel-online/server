package rule

import (
	"testing"

	modelx "github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
)

func hand(t *testing.T, input string) *GoujiHand {
	t.Helper()
	cards := modelx.Pokers{}
	for _, alias := range input {
		key := poker.GetKey(string(alias))
		if key == 0 {
			t.Fatalf("invalid alias %q in %q", alias, input)
		}
		cards = append(cards, modelx.Poker{Key: key})
	}
	return ParseGoujiHand(cards)
}

func TestParseGoujiHand(t *testing.T) {
	cases := []struct {
		input     string
		wantNil   bool
		mainVal   int
		mainCount int
		jokerVal  int
		jokerCnt  int
		gouji     bool
	}{
		{input: "3", mainVal: 3, mainCount: 1, gouji: false},
		{input: "0", mainVal: 10, mainCount: 1, gouji: false},    // 单10
		{input: "00000", mainVal: 10, mainCount: 5, gouji: true}, // 五个10
		{input: "0000", mainVal: 10, mainCount: 4, gouji: false}, // 四个10不是够级牌
		{input: "jjjj", mainVal: 11, mainCount: 4, gouji: true},  // 四个J
		{input: "jjj", mainVal: 11, mainCount: 3, gouji: false},
		{input: "qqq", mainVal: 12, mainCount: 3, gouji: true}, // 三个Q
		{input: "qq", mainVal: 12, mainCount: 2, gouji: false},
		{input: "kk", mainVal: 13, mainCount: 2, gouji: true}, // 两个K
		{input: "aa", mainVal: 14, mainCount: 2, gouji: true}, // 两个A
		{input: "a", mainVal: 14, mainCount: 1, gouji: false},
		{input: "2", mainVal: 15, mainCount: 1, gouji: true},                                // 单2
		{input: "22", mainVal: 15, mainCount: 2, gouji: true},                               // 对2
		{input: "s", mainVal: 16, mainCount: 1, gouji: true},                                // 小王
		{input: "x", mainVal: 17, mainCount: 1, gouji: true},                                // 大王
		{input: "ss", mainVal: 16, mainCount: 2, gouji: true},                               // 两张小王
		{input: "444s", mainVal: 4, mainCount: 3, jokerVal: 16, jokerCnt: 1, gouji: true},   // 挂小王
		{input: "6666xx", mainVal: 6, mainCount: 4, jokerVal: 17, jokerCnt: 2, gouji: true}, // 挂大王
		{input: "3344", wantNil: true},                                                      // 两种主体，非法
		{input: "44xs", wantNil: true},                                                      // 大小王混挂，非法
		{input: "44sx", wantNil: true},                                                      // 大小王混挂，非法
	}
	for _, c := range cases {
		h := hand(t, c.input)
		if c.wantNil {
			if h != nil {
				t.Errorf("%q: want nil, got %+v", c.input, *h)
			}
			continue
		}
		if h == nil {
			t.Errorf("%q: want hand, got nil", c.input)
			continue
		}
		if h.MainVal != c.mainVal || h.MainCount != c.mainCount || h.JokerVal != c.jokerVal || h.JokerCount != c.jokerCnt || h.Gouji != c.gouji {
			t.Errorf("%q: got %+v", c.input, *h)
		}
	}
}

func TestCanBeat(t *testing.T) {
	cases := []struct {
		cand string
		last string
		want bool
	}{
		// 普通：同张数比点数
		{"4", "3", true},
		{"3", "4", false},
		{"22", "aa", true},
		{"333", "kkk", false},
		{"kkk", "333", true},
		{"44", "22", false}, // 对A压不过对2
		{"44", "44", false},
		{"3333", "2222", false}, // 四个3压不过四个2
		{"2222", "3333", true},
		// 张数不同不能压
		{"44", "333", false},
		{"333", "44", false},
		// 挂王：只能被挂王压
		{"444x", "444s", true}, // 同主体挂大王压挂小王
		{"444s", "444x", false},
		{"555s", "444x", false}, // 主体555虽大，但挂小王压不了挂大王
		{"555x", "444s", true},
		{"555", "444x", false}, // 不挂王压不了挂王
		{"444", "444s", false},
		{"55s", "44s", true}, // 主体更大
		{"55x", "55s", true},
		{"44x", "555s", false}, // 张数不同
		// 王：x > s > 2
		{"x", "2", true},
		{"s", "2", true},
		{"2", "s", false},
		{"ss", "22", true},
		{"xx", "ss", true},
		{"x", "s", true},
	}
	for _, c := range cases {
		cand, last := hand(t, c.cand), hand(t, c.last)
		if cand == nil || last == nil {
			t.Fatalf("%q/%q: parse failed", c.cand, c.last)
		}
		if got := cand.CanBeat(last); got != c.want {
			t.Errorf("%s beat %s: want %v, got %v", c.cand, c.last, c.want, got)
		}
	}
}

func TestCanBeatAny(t *testing.T) {
	// 手牌：444 55 2 x 0，.last=333 → 有 444 可压
	h := handInput(t, "44455 2x0")
	last := hand(t, "333")
	if !CanBeatAny(h, last) {
		t.Error("444 should beat 333")
	}
	// last=2222（四个2，够级牌），手牌只有单王 → 压不过
	h2 := handInput(t, "34567890x")
	last2 := hand(t, "2222")
	if CanBeatAny(h2, last2) {
		t.Error("single joker should not beat 2222")
	}
	// last=999s（挂小王），手牌有 jjj x → 挂大王压
	h3 := handInput(t, "jjjxxaa")
	last3 := hand(t, "999s")
	if !CanBeatAny(h3, last3) {
		t.Error("jjj+x should beat 999+s")
	}
}

func handInput(t *testing.T, input string) modelx.Pokers {
	t.Helper()
	cards := modelx.Pokers{}
	for _, alias := range input {
		if alias == ' ' {
			continue
		}
		key := poker.GetKey(string(alias))
		if key == 0 {
			t.Fatalf("invalid alias %q", alias)
		}
		cards = append(cards, modelx.Poker{Key: key})
	}
	return cards
}
