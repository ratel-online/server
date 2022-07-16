package game

import (
	"math/rand"
	"sync"
	"time"
)

type Cycler struct {
	sync.Mutex
	elements  []string
	current   int
	direction int
}

func NewCycler(elements []string) *Cycler {
	return &Cycler{
		elements:  elements,
		current:   rand.New(rand.NewSource(time.Now().UnixNano())).Intn(len(elements)),
		direction: 1,
	}
}

func (c *Cycler) Current() string {
	return c.elements[c.current]
}

func (c *Cycler) ForEach(function func(string)) {
	for _, element := range c.elements {
		function(element)
	}
}

func (c *Cycler) Next() string {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	elementCount := len(c.elements)
	c.current = (c.current + c.direction + elementCount) % elementCount
	return c.elements[c.current]
}
