package color

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

type Color interface {
	Paint(string) string
	Paintf(string, ...interface{}) string
	String() string
}

type colorStruct struct {
	name          string
	colorFunction func(string, ...interface{}) string
}

func (c *colorStruct) Paint(text string) string {
	return c.colorFunction(text) + fmt.Sprintf("(%s)", c.name)
}

func (c *colorStruct) Paintf(text string, args ...interface{}) string {
	return c.colorFunction(text, args...) + fmt.Sprintf("(%s)", c.name)
}

func (c *colorStruct) String() string {
	return c.Paint(c.name)
}

var Red = &colorStruct{
	name:          "红",
	colorFunction: color.New(color.FgHiRed).SprintfFunc(),
}

var Yellow = &colorStruct{
	name:          "黄",
	colorFunction: color.New(color.FgHiYellow).SprintfFunc(),
}

var Green = &colorStruct{
	name:          "绿",
	colorFunction: color.New(color.FgHiGreen).SprintfFunc(),
}

var Blue = &colorStruct{
	name:          "蓝",
	colorFunction: color.New(color.FgHiCyan).SprintfFunc(),
}

var Stdout io.Writer = color.Output

var colors = map[string]Color{
	Red.name:    Red,
	Yellow.name: Yellow,
	Green.name:  Green,
	Blue.name:   Blue,
}

func ByName(name string) (Color, error) {
	color := colors[name]
	if color == nil {
		return nil, fmt.Errorf("invalid color '%s'", name)
	}
	return color, nil
}
