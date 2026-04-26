package fixtures

import "fmt"

// Shape is a sample interface for testing interface tracing.
type Shape interface {
	Area() float64
}

// Circle implements Shape.
type Circle struct {
	Radius float64
}

func (c Circle) Area() float64 {
	return 3.14 * c.Radius * c.Radius
}

// Square implements Shape.
type Square struct {
	Side float64
}

func (s Square) Area() float64 {
	return s.Side * s.Side
}

func Calculate(s Shape) {
	fmt.Println(s.Area())
}
