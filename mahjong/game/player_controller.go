package game

import (
	"sort"

	"github.com/ratel-online/server/mahjong/consts"
	"github.com/ratel-online/server/mahjong/event"
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

func (c *playerController) DarkGang(tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(consts.GANG, 0, tiles, false, false))
}

func (c *playerController) operation(op, target int, tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(op, target, tiles, true, false))
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
	event.PlayTile.Emit(event.PlayTilePayload{
		PlayerName: c.player.NickName(),
		Tile:       extraCard,
	})
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
	tiles := make([]int, 0, len(c.Hand())+1)
	tiles = append(tiles, pile.Top())
	tiles = append(tiles, c.Hand()...)
	op, tiles, err := c.player.PlayPrivileges(tiles, gameState)
	if err != nil {
		return 0, err
	}
	if len(tiles) == 0 {
		return c.Play(gameState)
	}
	c.AddTiles([]int{pile.DrawOneFromBehind()})
	c.operation(op, int(pile.LastPlayer().ID()), tiles)
	return c.Play(gameState)
}

func (c *playerController) Play(gameState State) (int, error) {
	tiles := c.Hand()
	sort.Ints(tiles)
	selectedTile, err := c.player.PlayMJ(tiles, gameState)
	if err != nil {
		return 0, err
	}
	c.hand.RemoveTile(selectedTile)
	return selectedTile, nil
}
