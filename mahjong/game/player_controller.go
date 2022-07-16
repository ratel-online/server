package game

import cwin "github.com/ratel-online/server/mahjong/win"

type playerController struct {
	player Player
	hand   *Hand
	show   *Hand
}

func newPlayerController(player Player) *playerController {
	return &playerController{
		player: player,
		hand:   NewHand(),
		show:   NewHand(),
	}
}

func (c *playerController) AddTiles(tiles []int) {
	c.hand.AddTiles(tiles)
	c.player.NotifyTilesDrawn(tiles)
}

func (c *playerController) Hand() []int {
	return c.hand.Tiles()
}

func (c *playerController) Name() string {
	return c.player.NickName()
}

func (c *playerController) ID() int64 {
	return c.player.PlayerID()
}

func (c *playerController) Play(gameState State, deck *Deck) (selectedTile int, win bool) {
	tile := deck.DrawOne()
	c.hand.AddTiles([]int{tile})
	if cwin.CanWin(c.Hand(), []int{}) {
		return 0, true
	}
	for {
		selectedTile = c.player.PlayMJ(gameState)
		c.hand.RemoveTile(selectedTile)
		return selectedTile, false
	}
}

func (c *playerController) tryTopDecking(gameState State, deck *Deck) int {
	extraTile := deck.DrawOne()
	c.AddTiles([]int{extraTile})
	c.hand.RemoveTile(extraTile)
	return extraTile
}
