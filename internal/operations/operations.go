// Package operations defines the only permitted changes to stacks A and B.
package operations

import (
	"fmt"

	"github.com/ernat-soltanbekov/swap-sort/internal/stack"
)

var Names = [...]string{"pa", "pb", "sa", "sb", "ss", "ra", "rb", "rr", "rra", "rrb", "rrr"}

type Machine struct{ A, B *stack.Stack }

func New(values []int64) *Machine {
	return &Machine{A: stack.New(values, len(values)), B: stack.New(nil, len(values))}
}
func (m *Machine) Sorted() bool { return m.B.Len() == 0 && m.A.Sorted() }
func push(from, to *stack.Stack) {
	if value, ok := from.Pop(); ok {
		if !to.Push(value) {
			panic("internal invariant: destination capacity exhausted")
		}
	}
}
func (m *Machine) Apply(name string) error {
	switch name {
	case "pa":
		push(m.B, m.A)
	case "pb":
		push(m.A, m.B)
	case "sa":
		m.A.Swap()
	case "sb":
		m.B.Swap()
	case "ss":
		m.A.Swap()
		m.B.Swap()
	case "ra":
		m.A.Rotate()
	case "rb":
		m.B.Rotate()
	case "rr":
		m.A.Rotate()
		m.B.Rotate()
	case "rra":
		m.A.Reverse()
	case "rrb":
		m.B.Reverse()
	case "rrr":
		m.A.Reverse()
		m.B.Reverse()
	default:
		return fmt.Errorf("unknown operation %q", name)
	}
	return nil
}

func Inverse(name string) string {
	switch name {
	case "pa":
		return "pb"
	case "pb":
		return "pa"
	case "ra":
		return "rra"
	case "rb":
		return "rrb"
	case "rr":
		return "rrr"
	case "rra":
		return "ra"
	case "rrb":
		return "rb"
	case "rrr":
		return "rr"
	default:
		return name // swaps undo themselves
	}
}
