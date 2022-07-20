package game

type Player interface {
	PlayerID() int64
	NickName() string
	Play(tiles []int, gameState State) (int, error)
	Take(tiles []int, gameState State) (int, []int, error)
}
