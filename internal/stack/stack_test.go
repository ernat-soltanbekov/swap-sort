package stack

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestRingAgainstSlice(t *testing.T) {
	for capacity := 1; capacity <= 32; capacity++ {
		s := New(nil, capacity)
		model := []int64{}
		random := rand.New(rand.NewSource(int64(capacity)))
		for step := 0; step < 10000; step++ {
			switch random.Intn(5) {
			case 0:
				value := random.Int63()
				got := s.Push(value)
				want := len(model) < capacity
				if got != want {
					t.Fatal("push capacity")
				}
				if want {
					model = append([]int64{value}, model...)
				}
			case 1:
				value, ok := s.Pop()
				if ok != (len(model) > 0) {
					t.Fatal("pop availability")
				}
				if ok {
					if value != model[0] {
						t.Fatal("pop value")
					}
					model = model[1:]
				}
			case 2:
				s.Swap()
				if len(model) > 1 {
					model[0], model[1] = model[1], model[0]
				}
			case 3:
				s.Rotate()
				if len(model) > 1 {
					model = append(model[1:], model[0])
				}
			case 4:
				s.Reverse()
				if len(model) > 1 {
					model = append([]int64{model[len(model)-1]}, model[:len(model)-1]...)
				}
			}
			if s.Len() != len(model) || fmt.Sprint(s.Values()) != fmt.Sprint(model) {
				t.Fatalf("capacity=%d step=%d got=%v want=%v", capacity, step, s.Values(), model)
			}
		}
	}
}
func TestSnapshotOwnershipAndBoundaries(t *testing.T) {
	input := []int64{1, 2, 3}
	s := New(input, 0)
	input[0] = 10
	snapshot := s.Values()
	snapshot[0] = 20
	if s.At(0) != 1 || !s.Sorted() {
		t.Fatal("stack shares external memory")
	}
	if s.Push(4) {
		t.Fatal("full push should fail without changing state")
	}
	for range 3 {
		s.Pop()
	}
	if _, ok := s.Pop(); ok {
		t.Fatal("empty pop")
	}
	if !s.Sorted() {
		t.Fatal("empty stack should be sorted")
	}
	s.Push(2)
	s.Swap()
	s.Rotate()
	s.Reverse()
	if s.At(0) != 2 {
		t.Fatal("singleton changed")
	}
}
