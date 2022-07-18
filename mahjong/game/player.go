package game

type Player interface {
	PlayerID() int64
	NickName() string
	PlayMJ(tiles []int, gameState State) (int, error)
	TakeMahjong(tiles []int, gameState State) (int, []int, error)
}
