package command

import "strings"

// Parse splits a slash command input into the command name and the raw
// argument string. It returns ("", "") if the input is not a valid slash
// command (i.e. does not start with "/").
//
// Examples:
//
//	Parse("/help")              → ("help", "")
//	Parse("/skill install foo") → ("skill", "install foo")
func Parse(input string) (name, args string) {
	if !strings.HasPrefix(input, "/") {
		return "", ""
	}
	input = input[1:] // strip leading "/"
	idx := strings.IndexByte(input, ' ')
	if idx < 0 {
		return input, ""
	}
	return input[:idx], input[idx+1:]
}
