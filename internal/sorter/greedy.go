package sorter

import "github.com/ernat-soltanbekov/swap-sort/internal/stack"

// greedy keeps B circularly descending while taking elements from A. It then
// merges B into circularly ascending A and turns A until its minimum is on top.
// Both stacks can rotate together, so all four direction pairs are evaluated.
func greedy(values []int64) []string {
	p := newPlanner(values)
	p.move("pb")
	p.move("pb")
	for p.A.Len() > 3 {
		targets := insertionTargets(p.B, len(values), false)
		a, b := choose(p.A, p.B, targets)
		p.rotate(a, b)
		p.move("pb")
	}
	// Order three values using A alone; B already contains live values.
	largest := 0
	for i := 1; i < 3; i++ {
		if p.A.At(i) > p.A.At(largest) {
			largest = i
		}
	}
	if largest == 0 {
		p.move("ra")
	} else if largest == 1 {
		p.move("rra")
	}
	if p.A.At(0) > p.A.At(1) {
		p.move("sa")
	}
	for p.B.Len() > 0 {
		targets := insertionTargets(p.A, len(values), true)
		b, a := choose(p.B, p.A, targets)
		p.rotate(a, b)
		p.move("pa")
	}
	minimum := 0
	for i := 1; i < p.A.Len(); i++ {
		if p.A.At(i) < p.A.At(minimum) {
			minimum = i
		}
	}
	p.rotate(shortest(minimum, p.A.Len()), 0)
	return p.ops
}

// For each rank, find the index it should precede. Sweeping the dense rank
// domain makes this linear instead of searching the target stack repeatedly.
func insertionTargets(destination *stack.Stack, count int, ascending bool) []int {
	positions := make([]int, count)
	for i := range positions {
		positions[i] = -1
	}
	minimum, maximum := count, -1
	for i := 0; i < destination.Len(); i++ {
		value := int(destination.At(i))
		positions[value] = i
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	targets := make([]int, count)
	if ascending {
		current := positions[minimum]
		for value := count - 1; value >= 0; value-- {
			targets[value] = current
			if positions[value] >= 0 {
				current = positions[value]
			}
		}
	} else {
		current := positions[maximum]
		for value := 0; value < count; value++ {
			targets[value] = current
			if positions[value] >= 0 {
				current = positions[value]
			}
		}
	}
	return targets
}

func choose(source, destination *stack.Stack, targets []int) (int, int) {
	bestCost, bestSource, bestTarget := int(^uint(0)>>1), 0, 0
	for i := 0; i < source.Len(); i++ {
		j := targets[int(source.At(i))]
		for _, a := range []int{i, i - source.Len()} {
			for _, b := range []int{j, j - destination.Len()} {
				cost := abs(a) + abs(b)
				if (a >= 0 && b >= 0) || (a <= 0 && b <= 0) {
					cost = max(abs(a), abs(b))
				}
				if cost < bestCost {
					bestCost, bestSource, bestTarget = cost, a, b
				}
			}
		}
	}
	return bestSource, bestTarget
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (p *planner) rotate(a, b int) {
	for a > 0 && b > 0 {
		p.move("rr")
		a--
		b--
	}
	for a < 0 && b < 0 {
		p.move("rrr")
		a++
		b++
	}
	for a > 0 {
		p.move("ra")
		a--
	}
	for a < 0 {
		p.move("rra")
		a++
	}
	for b > 0 {
		p.move("rb")
		b--
	}
	for b < 0 {
		p.move("rrb")
		b++
	}
}
