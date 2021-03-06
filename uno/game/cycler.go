package game

import "sync"

const (
	left  = -1
	right = 1
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
		current:   len(elements) - 1,
		direction: right,
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

func (c *Cycler) Reverse() {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	switch c.direction {
	case right:
		c.direction = left
	case left:
		c.direction = right
	}
}
