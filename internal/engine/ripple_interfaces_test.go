package engine

import (
	"testing"
)

func TestInterfacesDefined(t *testing.T) {
	// This test just ensures the types and interfaces are defined and compilable.
	var _ PropagationStrategy = nil
	var _ Validator = nil
}
