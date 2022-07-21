package database

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
	"github.com/feel-easy/uno/event"
	"github.com/feel-easy/uno/game"
	"github.com/ratel-online/server/consts"
)

type UnoGame struct {
	Room    *Room            `json:"room"`
	Players []int            `json:"players"`
	States  map[int]chan int `json:"states"`
	Game    *game.Game       `json:"game"`
}

func (ug *UnoGame) HavePlay(player *Player) bool {
	for _, id := range ug.Players {
		if id == int(player.ID) && player.online {
			return true
		}
	}
	return false
}

func (un *UnoGame) NeedExit() bool {
	return un.Room.Players <= 1
}

func (un *UnoGame) delete() {
	if un != nil {
		for _, state := range un.States {
			close(state)
		}
	}
}

type UnoPlayer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func NewUnoPlayer(p *Player) game.Player {
	return &UnoPlayer{
		ID:   int(p.ID),
		Name: p.Name,
	}
}

func (up *UnoPlayer) PlayerID() int {
	return up.ID
}

func (up *UnoPlayer) NickName() string {
	return up.Name
}

func contains(cards []card.Card, searchedCard card.Card) bool {
	for _, card := range cards {
		if card.Equal(searchedCard) {
			return true
		}
	}
	return false
}

func (up *UnoPlayer) NotifyCardsDrawn(cards []card.Card) {
	p := getPlayer(int64(up.ID))
	getPlayer(p.ID).WriteString(fmt.Sprintf("You drew %s!\n", cards))
}

func (up *UnoPlayer) NotifyNoMatchingCardsInHand(lastPlayedCard card.Card, hand []card.Card) {
	p := getPlayer(int64(up.ID))
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%s, none of your cards match %s! \n", p.Name, lastPlayedCard))
	buf.WriteString(fmt.Sprintf("Your hand is %s \n", hand))
	getPlayer(p.ID).WriteString(buf.String())
}

func (up *UnoPlayer) OnFirstCardPlayed(payload event.FirstCardPlayedPayload) {
	p := getPlayer(int64(up.ID))
	Broadcast(p.RoomID, fmt.Sprintf("First card is %s\n", payload.Card))
}

func (up *UnoPlayer) OnCardPlayed(payload event.CardPlayedPayload) {
	p := getPlayer(int64(up.ID))
	Broadcast(p.RoomID, fmt.Sprintf("%s played %s!\n", payload.PlayerName, payload.Card))
}

func (up *UnoPlayer) OnColorPicked(payload event.ColorPickedPayload) {
	p := getPlayer(int64(up.ID))
	Broadcast(p.RoomID, fmt.Sprintf("%s picked color %s!\n", payload.PlayerName, payload.Color))
}

func (up *UnoPlayer) OnPlayerPassed(payload event.PlayerPassedPayload) {
	p := getPlayer(int64(up.ID))
	Broadcast(p.RoomID, fmt.Sprintf("%s passed!\n", payload.PlayerName))
}

func (up *UnoPlayer) PickColor(gameState game.State) color.Color {
	p := getPlayer(int64(up.ID))
	for {
		p = getPlayer(p.ID)
		p.WriteString(fmt.Sprintf(
			"Select a color: %s, %s, %s or %s ? \n",
			color.Red,
			color.Yellow,
			color.Green,
			color.Blue,
		))
		colorName, err := p.AskForString(consts.PlayTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				return color.Red
			}
			p.WriteString(fmt.Sprintf("Unknown color '%s' \n", colorName))
			continue
		}
		chosenColor, err := color.ByName(strings.ToLower(colorName))
		if err != nil {
			p.WriteString(fmt.Sprintf("Unknown color '%s' \n", colorName))
			continue
		}
		return chosenColor
	}
}

func (up *UnoPlayer) Play(playableCards []card.Card, gameState game.State) (card.Card, error) {
	p := getPlayer(int64(up.ID))
	Broadcast(p.RoomID, fmt.Sprintf("It's %s turn! \n", p.Name), p.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("It's your turn, %s! \n", p.Name))
	buf.WriteString(gameState.String())
	p.WriteString(buf.String())
	runeSequence := runeSequence{}
	cardOptions := make(map[string]card.Card)
	for _, card := range playableCards {
		label := string(runeSequence.next())
		cardOptions[label] = card
	}
	cardSelectionLines := []string{"Select a card to play:"}
	for label, card := range cardOptions {
		cardSelectionLines = append(cardSelectionLines, fmt.Sprintf("%s %s", label, card))
	}
	cardSelectionMessage := strings.Join(cardSelectionLines, " \n ") + " \n "
	for {
		p = getPlayer(p.ID)
		p.WriteString(cardSelectionMessage)
		selectedLabel, err := p.AskForString(consts.PlayTimeout)
		if err != nil {
			if err == consts.ErrorsTimeout {
				selectedLabel = "A"
			} else {
				return nil, err
			}
		}
		selectedCard, found := cardOptions[strings.ToUpper(selectedLabel)]
		if !found {
			BroadcastChat(p, fmt.Sprintf("%s say: %s\n", p.Name, selectedLabel))
			continue
		}
		if !contains(playableCards, selectedCard) {
			p.WriteString(fmt.Sprintf("Cheat detected! Card %s is not in %s's hand! \n", selectedCard, p.Name))
			continue
		}
		return selectedCard, nil
	}
}
