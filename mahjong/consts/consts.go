package consts

const (
	_ int = iota
	CHI
	PENG
	GANG
)

var OpCodeData = map[int]string{
	CHI:  "吃",
	PENG: "碰",
	GANG: "杠",
}
