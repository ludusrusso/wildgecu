package context

import (
	"fmt"
	"strings"
)

type Type int

const (
	TypeUser     Type = iota
	TypeInternal Type = iota
	TypeMessages Type = iota
)

type ContextElement struct {
	Content string
	Type    Type
}

type Context struct {
	Elements []ContextElement
}

func (c Context) String() string {
	var res = ""
	for _, element := range c.Elements {
		res += fmt.Sprintf("%s\n\n", element.Content)
	}
	return strings.TrimSpace(res)
}
