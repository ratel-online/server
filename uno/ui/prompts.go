package ui

import (
	"fmt"
	"strings"

	"github.com/ratel-online/server/uno/card"
	"github.com/ratel-online/server/uno/card/color"
)

func PromptString(message string) string {
	for {
		Println(message)
		var input string
		_, err := fmt.Scanln(&input)
		if err != nil {
			Println("Invalid text input")
			continue
		}
		return input
	}
}

func promptInteger(message string) int {
	for {
		Println(message)
		var input int
		_, err := fmt.Scanln(&input)
		if err != nil {
			Println("Invalid number input")
			continue
		}
		return input
	}
}

func promptLowercaseString(message string) string {
	input := PromptString(message)
	return strings.ToLower(input)
}

func promptUppercaseString(message string) string {
	input := PromptString(message)
	return strings.ToUpper(input)
}

func PromptCardSelection(cards []card.Card) card.Card {
	runeSequence := runeSequence{}
	cardOptions := make(map[string]card.Card)
	for _, card := range cards {
		label := string(runeSequence.next())
		cardOptions[label] = card
	}

	cardSelectionLines := []string{"Select a card to play:"}
	for label, card := range cardOptions {
		cardSelectionLines = append(cardSelectionLines, fmt.Sprintf("%s (enter %s)", card, label))
	}
	cardSelectionMessage := strings.Join(cardSelectionLines, "\n")

	for {
		selectedLabel := promptUppercaseString(cardSelectionMessage)
		selectedCard, found := cardOptions[selectedLabel]
		if !found {
			Printfln("No card assigned to '%s'", selectedLabel)
			continue
		}
		return selectedCard
	}
}

func PromptColor() color.Color {
	colorMessage := fmt.Sprintf(
		"Select a color: '%s', '%s', '%s' or '%s'?",
		color.Red,
		color.Yellow,
		color.Green,
		color.Blue,
	)
	for {
		colorName := promptLowercaseString(colorMessage)
		chosenColor, err := color.ByName(colorName)
		if err != nil {
			Printfln("Unknown color '%s'", colorName)
			continue
		}
		return chosenColor
	}
}

func PromptIntegerInRange(minimum int, maximum int, message string) int {
	for {
		input := promptInteger(message)
		if input < minimum || input > maximum {
			Printfln("Input out of range (minimum: %d, maximum: %d)", minimum, maximum)
			continue
		}
		return input
	}
}
