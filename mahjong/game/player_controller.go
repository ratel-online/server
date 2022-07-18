package game

import (
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

func (c *playerController) DarkGang(tile int) {
	c.showCards = append(c.showCards, NewShowCard(consts.GANG, 0, []int{tile, tile, tile, tile}, false, false))
}

func (c *playerController) operation(op, target int, tiles []int) {
	c.showCards = append(c.showCards, NewShowCard(op, target, tiles, true, false))
}

func (c *playerController) GetShowCard() []*ShowCard {
	return c.showCards
}

func (c *playerController) FindShowCard(id int) *ShowCard {
	for _, sc := range c.showCards {
		if util.IntInSlice(id, sc.tiles) {
			return sc
		}
	}
	return nil
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

func (c *playerController) TryBottomDecking(deck *Deck) {
	extraCard := deck.BottomDrawOne()
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
func (c *playerController) LastTile() int {
	return c.hand.Tiles()[len(c.hand.Tiles())-1]
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

func (c *playerController) TakeMahjong(gameState State, deck *Deck, pile *Pile) (int, bool, error) {
	tiles := make([]int, 0, len(c.Hand())+1)
	tiles = append(tiles, c.Hand()...)
	tiles = append(tiles, pile.Top())
	op, tiles, err := c.player.TakeMahjong(tiles, gameState)
	if err != nil {
		return op, false, err
	}
	if len(tiles) == 0 {
		switch op {
		case consts.CHI:
			c.TryTopDecking(deck)
		case consts.PENG:
			if gameState.OriginallyPlayer.ID() == c.ID() {
				c.TryTopDecking(deck)
			}
		case consts.GANG:
			if gameState.OriginallyPlayer.ID() == c.ID() {
				c.TryTopDecking(deck)
			}
		}
		pile.AddSayNoPlayer(c)
		return op, false, nil
	}
	c.AddTiles([]int{pile.BottomDrawOne()})
	c.operation(op, int(pile.LastPlayer().ID()), tiles)
	return op, true, nil
}

func (c *playerController) Play(gameState State) (int, error) {
	selectedTile, err := c.player.PlayMJ(c.Hand(), gameState)
	if err != nil {
		return 0, err
	}
	c.hand.RemoveTile(selectedTile)
	return selectedTile, nil
}
