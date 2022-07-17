package game

type Player interface {
	PlayerID() int64
	NickName() string
	PlayMJ(tiles []int, gameState State) (int, error)
	PlayPrivileges(tiles []int, gameState State) (int, []int, error)
	NotifyTilesDrawn(drawnTiles []int)
}
