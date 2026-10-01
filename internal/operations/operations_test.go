package operations

import (
	"fmt"
	"testing"
)

func TestAllElevenOperations(t *testing.T) {
	// Starting state: A=[2 1 3], B=[4 5 6].
	cases := []struct {
		op   string
		a, b string
	}{
		{"pa", "[4 2 1 3]", "[5 6]"}, {"pb", "[1 3]", "[2 4 5 6]"},
		{"sa", "[1 2 3]", "[4 5 6]"}, {"sb", "[2 1 3]", "[5 4 6]"}, {"ss", "[1 2 3]", "[5 4 6]"},
		{"ra", "[1 3 2]", "[4 5 6]"}, {"rb", "[2 1 3]", "[5 6 4]"}, {"rr", "[1 3 2]", "[5 6 4]"},
		{"rra", "[3 2 1]", "[4 5 6]"}, {"rrb", "[2 1 3]", "[6 4 5]"}, {"rrr", "[3 2 1]", "[6 4 5]"},
	}
	for _, test := range cases {
		t.Run(test.op, func(t *testing.T) {
			m := New([]int64{6, 5, 4, 2, 1, 3})
			for range 3 {
				m.Apply("pb")
			}
			if err := m.Apply(test.op); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(m.A.Values()) != test.a || fmt.Sprint(m.B.Values()) != test.b {
				t.Fatal(m.A.Values(), m.B.Values())
			}
			m.Apply(Inverse(test.op))
			if fmt.Sprint(m.A.Values()) != "[2 1 3]" || fmt.Sprint(m.B.Values()) != "[4 5 6]" {
				t.Fatal("inverse failed")
			}
		})
	}
}
func TestNoOpsAndCombinedPartialStacks(t *testing.T) {
	for _, values := range [][]int64{nil, {1}} {
		for _, op := range Names {
			m := New(values)
			if err := m.Apply(op); err != nil {
				t.Fatal(err)
			}
			if m.A.Len()+m.B.Len() != len(values) {
				t.Fatal("lost value")
			}
		}
	}
	for _, op := range []string{"ss", "rr", "rrr"} {
		m := New([]int64{3, 1, 2})
		m.Apply(op)
		if fmt.Sprint(m.A.Values()) == "[3 1 2]" {
			t.Fatal("empty B prevented operation on A")
		}
	}
	m := New([]int64{1})
	m.Apply("pb")
	if m.Sorted() {
		t.Fatal("nonempty B accepted")
	}
	if m.Apply("PA") == nil {
		t.Fatal("unknown operation accepted")
	}
}
