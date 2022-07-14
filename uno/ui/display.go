package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ratel-online/server/uno/card/color"
)

func Printfln(format string, args ...interface{}) {
	Println(fmt.Sprintf(format, args...))
}

func Printlns(lines []string) {
	Println(strings.Join(lines, "\n"))
}

func Println(args ...interface{}) {
	fmt.Fprintln(color.Stdout, args...)
	time.Sleep(1 * time.Second)
}
