package game

import (
	"math/rand"
	"sync"

	"github.com/feel-easy/uno/card"
	"github.com/feel-easy/uno/card/color"
)

type Deck struct {
	sync.Mutex
	cards []card.Card
}

func NewDeck() *Deck {
	deck := &Deck{}
	fillDeck(deck)
	return deck
}

func (d *Deck) DrawOne() card.Card {
	return d.Draw(1)[0]
}

func (d *Deck) Draw(amount int) []card.Card {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()
	if len(d.cards) < amount {
		fillDeck(d)
	}
	cards := d.cards[0:amount]
	d.cards = d.cards[amount:]
	return cards
}

func fillDeck(deck *Deck) {
	cards := make([]card.Card, 0, 108)

	cards = append(cards, createBlackCards()...)
	cards = append(cards, createColorCards(color.Red)...)
	cards = append(cards, createColorCards(color.Yellow)...)
	cards = append(cards, createColorCards(color.Green)...)
	cards = append(cards, createColorCards(color.Blue)...)

	shuffleCards(cards)

	deck.cards = append(deck.cards, cards...)
}

func createColorCards(cardColor color.Color) []card.Card {
	zeroCard := card.NewNumberCard(cardColor, 0)
	skipCard := card.NewSkipCard(cardColor)
	reverseCard := card.NewReverseCard(cardColor)
	drawTwoCard := card.NewDrawTwoCard(cardColor)

	cards := []card.Card{
		zeroCard,
		skipCard, skipCard,
		reverseCard, reverseCard,
		drawTwoCard, drawTwoCard,
	}

	for number := 1; number <= 9; number++ {
		numberCard := card.NewNumberCard(cardColor, number)
		cards = append(cards, numberCard, numberCard)
	}

	return cards
}

func createBlackCards() []card.Card {
	wildCard := card.NewWildCard()
	wildDrawFourCard := card.NewWildDrawFourCard()

	return []card.Card{
		wildCard, wildCard, wildCard, wildCard,
		wildDrawFourCard, wildDrawFourCard, wildDrawFourCard, wildDrawFourCard,
	}
}

func shuffleCards(cards []card.Card) {
	rand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
}
