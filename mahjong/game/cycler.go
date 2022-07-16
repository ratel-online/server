package game

import (
	"sync"
)

type Cycler struct {
	sync.Mutex
	elements  []int64
	current   int
	direction int
}

func NewCycler(elements []int64) *Cycler {
	return &Cycler{
		elements:  elements,
		current:   len(elements) - 1,
		direction: 1,
	}
}

func (c *Cycler) Current() int64 {
	return c.elements[c.current]
}

func (c *Cycler) ForEach(function func(int64)) {
	for _, element := range c.elements {
		function(element)
	}
}

func (c *Cycler) Next() int64 {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	elementCount := len(c.elements)
	c.current = (c.current + c.direction + elementCount) % elementCount
	return c.elements[c.current]
}
