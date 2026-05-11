package main

import (
	"github.com/Rogercode97/scouter/tests/fixtures/semantic_ripple/domain"
	"github.com/Rogercode97/scouter/tests/fixtures/semantic_ripple/infra"
)

func main() {
	var l domain.Logger = &infra.ConsoleLogger{}
	l.Log("hello")
}
