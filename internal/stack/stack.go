// Package stack implements a bounded double-ended queue, with index zero on top.
package stack

// Stack uses a ring so pushing and rotating do not copy the entire stack.
// A pair of stacks each gets room for all values: pushes can never overflow.
type Stack struct {
	values     []int64
	head, size int
}

func New(values []int64, capacity int) *Stack {
	if capacity < len(values) {
		capacity = len(values)
	}
	if capacity < 1 {
		capacity = 1
	}
	s := &Stack{values: make([]int64, capacity), size: len(values)}
	copy(s.values, values)
	return s
}

func (s *Stack) Len() int { return s.size }
func (s *Stack) At(index int) int64 {
	if index < 0 || index >= s.size {
		panic("stack index out of range")
	}
	return s.values[(s.head+index)%len(s.values)]
}
func (s *Stack) Values() []int64 {
	values := make([]int64, s.size)
	for i := range values {
		values[i] = s.At(i)
	}
	return values
}
func (s *Stack) Push(value int64) bool {
	if s.size == len(s.values) {
		return false
	}
	s.head = (s.head + len(s.values) - 1) % len(s.values)
	s.values[s.head] = value
	s.size++
	return true
}
func (s *Stack) Pop() (int64, bool) {
	if s.size == 0 {
		return 0, false
	}
	value := s.values[s.head]
	s.head = (s.head + 1) % len(s.values)
	s.size--
	return value, true
}
func (s *Stack) Swap() {
	if s.size < 2 {
		return
	}
	next := (s.head + 1) % len(s.values)
	s.values[s.head], s.values[next] = s.values[next], s.values[s.head]
}
func (s *Stack) Rotate() {
	if s.size < 2 {
		return
	}
	value, _ := s.Pop()
	s.values[(s.head+s.size)%len(s.values)] = value
	s.size++
}
func (s *Stack) Reverse() {
	if s.size < 2 {
		return
	}
	value := s.At(s.size - 1)
	s.size--
	s.Push(value)
}
func (s *Stack) Sorted() bool {
	for i := 1; i < s.size; i++ {
		if s.At(i-1) >= s.At(i) {
			return false
		}
	}
	return true
}
