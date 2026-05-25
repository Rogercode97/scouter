package main

import (
	"fmt"
	"reflect"
	"github.com/odvcencio/gotreesitter"
)

func main() {
	var n *gotreesitter.Node
	t := reflect.TypeOf(n)
	for i := 0; i < t.NumMethod(); i++ {
		fmt.Println(t.Method(i).Name)
	}
}
