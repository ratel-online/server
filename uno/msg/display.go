package msg

import (
	"fmt"
	"strings"
)

func Sprintfln(format string, args ...interface{}) string {
	return Sprintln(fmt.Sprintf(format, args...))
}

func Sprintlns(lines []string) string {
	return Sprintln(strings.Join(lines, "\n"))
}

func Sprintln(args ...interface{}) string {
	return fmt.Sprintln(args...)
}
