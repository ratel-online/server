package game

type Player interface {
	PlayerID() int64
	NickName() string
	PlayMJ(gameState State) int
	NotifyTilesDrawn(drawnTiles []int)
}
