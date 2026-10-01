// Package sorter plans instructions without changing the caller's input.
package sorter

import (
	"fmt"
	"math/bits"

	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
)

type Result struct {
	Operations []string
	Strategy   string
	Candidates []Candidate
}
type Candidate struct {
	Strategy string
	Count    int
}

type planner struct {
	*operations.Machine
	ops []string
}

func newPlanner(values []int64) *planner { return &planner{Machine: operations.New(values)} }
func (p *planner) move(name string) {
	if err := p.Apply(name); err != nil {
		panic(err)
	}
	p.ops = append(p.ops, name)
}

// Sort uses exact shortest paths through seven values and chooses the shortest
// of several deterministic plans for larger inputs. Every plan uses only the
// eleven stack instructions. Large-input plans are heuristics, not a proof of
// global optimality.
func Sort(values []int64) Result {
	if operations.New(values).Sorted() {
		return Result{Strategy: "already sorted"}
	}
	if ops, ok := simplePlan(values); ok {
		return Result{Operations: ops, Strategy: "direct swap or circular rotation", Candidates: []Candidate{{"direct swap or circular rotation", len(ops)}}}
	}
	ranks := rank(values)
	if len(ranks) <= 7 {
		ops := exact(ranks)
		return Result{Operations: ops, Strategy: "exact breadth-first search", Candidates: []Candidate{{"exact breadth-first search", len(ops)}}}
	}
	result := Result{}
	add := func(name string, ops []string) {
		ops = simplify(ops)
		result.Candidates = append(result.Candidates, Candidate{name, len(ops)})
		if result.Operations == nil || len(ops) < len(result.Operations) {
			result.Operations, result.Strategy = ops, name
		}
	}
	add("binary radix", radix(ranks))
	// Quadratic planning is bounded; large lists retain the O(n log n) radix
	// fallback. The ring stacks keep each generated instruction O(1).
	if len(ranks) <= 512 {
		add("greedy paired rotations", greedy(ranks))
		root := 1
		for root*root < len(ranks) {
			root++
		}
		for _, width := range []int{root, root + root/3, root + root/2, 2 * root} {
			add(fmt.Sprintf("rank bands (width %d)", width), bands(ranks, width))
		}
	}
	return result
}

// rank orders fixed-width signed keys with eight byte-counting passes, never
// a comparison sort. Replacing values by ranks preserves their relative order
// without subtracting integers (which would overflow at the signed limits).
func rank(values []int64) []int64 {
	keys := append([]int64(nil), values...)
	scratch := make([]int64, len(keys))
	for shift := uint(0); shift < 64; shift += 8 {
		var counts [256]int
		for _, value := range keys {
			counts[byte((uint64(value)^(1<<63))>>shift)]++
		}
		total := 0
		for i, count := range counts {
			counts[i], total = total, total+count
		}
		for _, value := range keys {
			bucket := byte((uint64(value) ^ (1 << 63)) >> shift)
			scratch[counts[bucket]] = value
			counts[bucket]++
		}
		keys, scratch = scratch, keys
	}
	positions := make(map[int64]int64, len(keys))
	for i, value := range keys {
		positions[value] = int64(i)
	}
	result := make([]int64, len(values))
	for i, value := range values {
		result[i] = positions[value]
	}
	return result
}

func radix(values []int64) []string {
	p := newPlanner(values)
	for bit := 0; bit < bits.Len(uint(len(values)-1)); bit++ {
		for remaining := p.A.Len(); remaining > 0; remaining-- {
			if p.A.At(0)&(1<<bit) == 0 {
				p.move("pb")
			} else {
				p.move("ra")
			}
		}
		for p.B.Len() > 0 {
			p.move("pa")
		}
	}
	return p.ops
}

func bands(values []int64, width int) []string {
	p := newPlanner(values)
	for p.A.Len() > 0 {
		switch {
		case int(p.A.At(0)) <= p.B.Len():
			p.move("pb")
			p.move("rb")
		case int(p.A.At(0)) <= p.B.Len()+width:
			p.move("pb")
		default:
			p.move("ra")
		}
	}
	for p.B.Len() > 0 {
		largest := 0
		for i := 1; i < p.B.Len(); i++ {
			if p.B.At(i) > p.B.At(largest) {
				largest = i
			}
		}
		p.rotate(0, shortest(largest, p.B.Len()))
		p.move("pa")
	}
	return p.ops
}

func shortest(index, length int) int {
	if index <= length/2 {
		return index
	}
	return index - length
}

// simplify combines adjacent moves on independent stacks and removes pairs
// that undo each other. Pushes are deliberately left alone: empty-stack pushes
// are no-ops, so cancellation would not be safe for arbitrary sequences.
func simplify(ops []string) []string {
	result := make([]string, 0, len(ops))
	for _, op := range ops {
		if len(result) == 0 {
			result = append(result, op)
			continue
		}
		previous := result[len(result)-1]
		if op != "pa" && op != "pb" && previous == operations.Inverse(op) {
			result = result[:len(result)-1]
			continue
		}
		combined := ""
		switch previous + ":" + op {
		case "ra:rb", "rb:ra":
			combined = "rr"
		case "rra:rrb", "rrb:rra":
			combined = "rrr"
		case "sa:sb", "sb:sa":
			combined = "ss"
		}
		if combined != "" {
			result[len(result)-1] = combined
		} else {
			result = append(result, op)
		}
	}
	return result
}

// Recognize common nearly sorted inputs before running a general planner.
func simplePlan(values []int64) ([]string, bool) {
	// Exact BFS already handles the small cases and proves their optimality.
	if len(values) <= 7 {
		return nil, false
	}
	p := newPlanner(values)
	p.move("sa")
	if p.Sorted() {
		return p.ops, true
	}
	p = newPlanner(values)
	minimum, descents := 0, 0
	for i, value := range values {
		if value < values[minimum] {
			minimum = i
		}
		if value > values[(i+1)%len(values)] {
			descents++
		}
	}
	if descents != 1 {
		return nil, false
	}
	p.rotate(shortest(minimum, len(values)), 0)
	return p.ops, true
}
