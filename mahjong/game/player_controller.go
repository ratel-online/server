package game

import (
	mconsts "github.com/ratel-online/server/mahjong/consts"
	"github.com/ratel-online/server/mahjong/util"
)

type playerController struct {
	player    Player
	hand      *Hand
	showCards []*ShowCard
}

func newPlayerController(player Player) *playerController {
	return &playerController{
		player:    player,
		hand:      NewHand(),
		showCards: make([]*ShowCard, 0, 5),
	}
}

func (c *playerController) Chi(target int, tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(mconsts.CHI, target, tiles, false))
}

func (c *playerController) Peng(target int, tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(mconsts.PENG, target, tiles, false))
}

func (c *playerController) Gang(target int, tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(mconsts.GANG, target, tiles, false))
}

func (c *playerController) GetShowCard() []*ShowCard {
	return c.showCards
}

func (c *playerController) GetShowCardTiles() []int {
	ret := make([]int, 0, len(c.showCards)*4)
	for _, t := range c.showCards {
		ret = append(ret, t.tiles...)
	}
	return ret
}

func (c *playerController) AddTiles(tiles []int) {
	c.hand.AddTiles(tiles)
	c.player.NotifyTilesDrawn(tiles)
}

func (c *playerController) TryTopDecking(deck *Deck) {
	extraCard := deck.DrawOne()
	c.AddTiles([]int{extraCard})
}

func (c *playerController) Hand() []int {
	tiles := c.Tiles()
	return util.SliceDel(tiles, c.GetShowCardTiles()...)
}

func (c *playerController) Tiles() []int {
	return c.hand.Tiles()
}

func (c *playerController) Name() string {
	return c.player.NickName()
}

func (c *playerController) ID() int64 {
	return c.player.PlayerID()
}
func (c *playerController) Player() *Player {
	return &c.player
}

func (c *playerController) PlayPrivileges(gameState State, pile *Pile) (int, error) {
	c.AddTiles([]int{pile.DrawOne()})
	tiles, err := c.player.PlayPrivileges(c.Hand(), gameState)
	if err != nil {
		return 0, err
	}
	c.Chi(int(pile.LastPlayer().ID()), tiles)
	return c.Play(gameState)
}

func (c *playerController) Play(gameState State) (int, error) {
	selectedTile, err := c.player.PlayMJ(c.Hand(), gameState)
	if err != nil {
		return 0, err
	}
	c.hand.RemoveTile(selectedTile)
	return selectedTile, nil
}
