package database

import (
	"strings"
	"unicode"
)

// ============================================================================
// 文本归一化 + 脏词屏蔽
//
// core 里的 Desensitize 是纯子串替换，"傻 逼"、"s.b"、"煞笔"、"SB" 全部漏过。
// 这里先归一化再匹配：
//  1. 全角转半角、常见混淆字符折叠（@ -> a，0 -> o，1 -> i ...）；
//  2. 去掉空白、标点、符号、emoji、零宽字符，只保留字母与数字；
//  3. 用归一化后的词表匹配，命中后按"原始 rune 下标"回填 '*'，保留原排版。
//
// 归一化文本与原始文本之间维持下标映射，所以 "傻 逼" 会整段变成 "***"，
// 而不是把空格也吃掉后错位。
// ============================================================================

// profanityWords 全部是"归一化形态"（小写、无分隔符）。
// 纯 ASCII 的词额外要求词边界，避免命中普通英文单词内部的片段。
var profanityWords = []string{
	// 中文脏话
	"傻逼", "傻b", "傻比", "傻批", "傻屄", "傻叉", "傻x", "傻缺", "傻狗", "傻鸟",
	"煞笔", "煞逼", "沙比", "沙雕",
	"脑残", "智障", "弱智", "白痴", "白吃", "废物", "废柴",
	"杂种", "贱人", "贱货", "婊子", "表子", "骚货",
	"狗东西", "狗日的", "狗娘养", "狗杂种",
	"你妈逼", "你麻痹", "你妈的", "草泥马", "操你妈", "艹你妈",
	"日你妈", "干你妈", "妈了个", "他妈的",
	"去死", "死全家", "滚出去",
	// 拼音/变体
	"shabi", "shab", "sabi", "sbi", "sb", "jb", "caonima", "cnm", "nmsl", "nmb",
	"nima", "tmd", "tnnd", "rnm", "wrnm", "zhizhang", "ruozhi",
	"naocan", "baichi", "feiwu", "jianren", "biaozi", "zazhong",
	"gouzazhong", "qunimade",
	// 英文
	"fuck", "fucking", "fucker", "motherfucker", "shit", "bullshit",
	"bitch", "asshole", "bastard", "idiot", "moron", "retard",
	"stfu", "kys", "dickhead", "cunt", "whore",
}

// homoglyphs 常见混淆字符折叠，挡住 "@fck"、"傻b1" 这类变体。
var homoglyphs = map[rune]rune{
	'@': 'a',
	'0': 'o',
	'1': 'i',
	'3': 'e',
	'4': 'a',
	'5': 's',
	'$': 's',
	'!': 'i',
}

// normalized 归一化文本，orig[i] 是 text[i] 在原文中的 rune 下标。
type normalized struct {
	text []rune
	orig []int
}

func normalizeMessage(s string) normalized {
	runes := []rune(s)
	n := normalized{
		text: make([]rune, 0, len(runes)),
		orig: make([]int, 0, len(runes)),
	}
	for i, r := range runes {
		// 全角 ASCII -> 半角
		if r >= 0xFF01 && r <= 0xFF5E {
			r -= 0xFEE0
		}
		if h, ok := homoglyphs[r]; ok {
			r = h
		}
		switch {
		case r == 0x3000, r == 0x00A0:
			continue
		case unicode.IsSpace(r), unicode.IsControl(r):
			continue
		case r >= 0x200B && r <= 0x200F, r == 0xFEFF, r == 0x2028, r == 0x2029:
			continue
		}
		r = unicode.ToLower(r)
		// 标点、符号、emoji 一律视为分隔符（等同"傻.逼"里的点）
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		n.text = append(n.text, r)
		n.orig = append(n.orig, i)
	}
	return n
}

// normalizeText 返回归一化后的文本，主要用于测试与日志。
func normalizeText(s string) string {
	return string(normalizeMessage(s).text)
}

func isASCIIWord(w string) bool {
	for _, r := range w {
		if r < 'a' || r > 'z' {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func isASCIIAlnum(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
}

// indexRunes 在 hay 中从 from 开始找 needle，返回 rune 下标，找不到返回 -1。
func indexRunes(hay, needle []rune, from int) int {
	if len(needle) == 0 || from < 0 {
		return -1
	}
	for i := from; i+len(needle) <= len(hay); i++ {
		matched := true
		for j := range needle {
			if hay[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

// boundaryOK 纯 ASCII 的词要求两侧不是字母数字，避免 "asb" 之类误伤。
func boundaryOK(text []rune, start, end int) bool {
	if start > 0 && isASCIIAlnum(text[start-1]) {
		return false
	}
	if end < len(text) && isASCIIAlnum(text[end]) {
		return false
	}
	return true
}

// maskProfanity 把命中脏词的片段替换为 '*'，返回掩码后的文本和命中次数。
// 命中次数用于行为计数（一条消息只算一次违规）。
func maskProfanity(s string) (string, int) {
	runes := []rune(s)
	if len(runes) == 0 {
		return s, 0
	}
	n := normalizeMessage(s)
	if len(n.text) == 0 {
		return s, 0
	}

	mark := make([]bool, len(runes))
	hits := 0
	for _, word := range profanityWords {
		needle := []rune(word)
		ascii := isASCIIWord(word)
		from := 0
		for {
			start := indexRunes(n.text, needle, from)
			if start < 0 {
				break
			}
			end := start + len(needle)
			from = start + 1
			if ascii && !boundaryOK(n.text, start, end) {
				continue
			}
			for k := start; k < end; k++ {
				mark[n.orig[k]] = true
			}
			hits++
		}
	}
	if hits == 0 {
		return s, 0
	}
	for i := range runes {
		if mark[i] {
			runes[i] = '*'
		}
	}
	return string(runes), hits
}

// sanitizeChat 是聊天内容的统一清洗：先屏蔽脏词，再做原有的词表替换。
// 返回清洗后的文本与违规命中数（>0 表示本次要记一次违规）。
func sanitizeChat(msg string) (string, int) {
	masked, hits := maskProfanity(msg)
	masked = strings.TrimSpace(masked)
	return masked, hits
}
