package game_test

import (
	"fmt"
	"testing"

	"github.com/ratel-online/server/uno/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrent(t *testing.T) {
	cycler := game.NewCycler([]string{"A", "B", "C", "D"})
	assert.Equal(t, "D", cycler.Current())
	cycler.Next()
	assert.Equal(t, "A", cycler.Current())
	cycler.Next()
	assert.Equal(t, "B", cycler.Current())
	cycler.Reverse()
	cycler.Next()
	assert.Equal(t, "A", cycler.Current())
	cycler.Next()
	assert.Equal(t, "D", cycler.Current())
	cycler.Next()
	assert.Equal(t, "C", cycler.Current())
	cycler.Reverse()
	cycler.Next()
	assert.Equal(t, "D", cycler.Current())
	cycler.Next()
	assert.Equal(t, "A", cycler.Current())
}

func TestForEach(t *testing.T) {
	cycler := game.NewCycler([]string{"A", "B", "C", "D"})

	var results []string
	cycler.ForEach(func(element string) {
		results = append(results, fmt.Sprintf("called for %s", element))
	})

	require.Equal(t, []string{
		"called for A",
		"called for B",
		"called for C",
		"called for D",
	}, results)
}

func TestNext(t *testing.T) {
	cycler := game.NewCycler([]string{"A", "B", "C", "D"})
	assert.Equal(t, "A", cycler.Next())
	assert.Equal(t, "B", cycler.Next())
	assert.Equal(t, "C", cycler.Next())
	assert.Equal(t, "D", cycler.Next())
	assert.Equal(t, "A", cycler.Next())
}

func TestReverse(t *testing.T) {
	cycler := game.NewCycler([]string{"A", "B", "C", "D"})
	assert.Equal(t, "A", cycler.Next())
	assert.Equal(t, "B", cycler.Next())
	cycler.Reverse()
	assert.Equal(t, "A", cycler.Next())
	assert.Equal(t, "D", cycler.Next())
	assert.Equal(t, "C", cycler.Next())
	cycler.Reverse()
	assert.Equal(t, "D", cycler.Next())
	assert.Equal(t, "A", cycler.Next())
}
