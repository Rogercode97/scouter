package main

import (
	"fmt"
	"github.com/Rogercode97/scouter/internal/utils"
)

func main() {
	tsDoc := `/**
 * Greeter class
 * second line
 */`
	cleaned := utils.CleanComment(tsDoc)
	fmt.Printf("Input:\n%s\n", tsDoc)
	fmt.Printf("Output (quoted): %q\n", cleaned)
	fmt.Printf("Output (raw):\n%s\n", cleaned)

	expected := "Greeter class\nsecond line"
	if cleaned != expected {
		fmt.Printf("FAILED: Expected %q, got %q\n", expected, cleaned)
	} else {
		fmt.Printf("PASSED\n")
	}
}
