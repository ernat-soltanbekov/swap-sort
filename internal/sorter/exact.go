package sorter

import (
	"sync"

	"github.com/ernat-soltanbekov/swap-sort/internal/operations"
)

// A state key stores A's length, then A followed by B. Ranks fit in one byte.
// Starting at the goal, breadth-first search visits all states by distance.
// Each discovered state records the inverse move that leads closer to the goal.
type step struct {
	next      string
	operation string
}

var tables [8]struct {
	once  sync.Once
	paths map[string]step
}

func key(m *operations.Machine) string {
	data := []byte{byte(m.A.Len())}
	for _, value := range m.A.Values() {
		data = append(data, byte(value))
	}
	for _, value := range m.B.Values() {
		data = append(data, byte(value))
	}
	return string(data)
}
func restore(data string) *operations.Machine {
	values := make([]int64, len(data)-1)
	for i := range values {
		values[i] = int64(data[i+1])
	}
	m := operations.New(values)
	// Preserve B's top-to-bottom order while moving the split to its position.
	for m.A.Len() > int(data[0]) {
		m.A.Reverse()
		_ = m.Apply("pb")
	}
	return m
}

func buildTable(n int) map[string]step {
	goal := make([]int64, n)
	for i := range goal {
		goal[i] = int64(i)
	}
	start := key(operations.New(goal))
	paths := map[string]step{start: {}}
	queue := []string{start}
	for index := 0; index < len(queue); index++ {
		current := queue[index]
		for _, op := range operations.Names {
			m := restore(current)
			_ = m.Apply(op)
			neighbor := key(m)
			if _, seen := paths[neighbor]; seen {
				continue
			}
			paths[neighbor] = step{current, operations.Inverse(op)}
			queue = append(queue, neighbor)
		}
	}
	return paths
}

func exact(ranks []int64) []string {
	table := &tables[len(ranks)]
	table.once.Do(func() { table.paths = buildTable(len(ranks)) })
	state := key(operations.New(ranks))
	var result []string
	for {
		entry := table.paths[state]
		if entry.operation == "" {
			return result
		}
		result = append(result, entry.operation)
		state = entry.next
	}
}
