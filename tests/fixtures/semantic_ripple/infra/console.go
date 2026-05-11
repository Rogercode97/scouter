package infra

import "fmt"

type ConsoleLogger struct{}

func (c *ConsoleLogger) Log(msg string) {
	fmt.Println(msg)
}
